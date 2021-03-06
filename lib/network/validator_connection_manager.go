package network

import (
	"errors"
	"net"
	"net/http"
	"sync"
	"time"

	"boscoin.io/sebak/lib/ballot"
	"boscoin.io/sebak/lib/common"
	"boscoin.io/sebak/lib/node"
	logging "github.com/inconshreveable/log15"
)

type ValidatorConnectionManager struct {
	sync.RWMutex

	localNode *node.LocalNode
	network   Network
	policy    ballot.VotingThresholdPolicy

	validators map[ /* node.Address() */ string]*node.Validator
	clients    map[ /* node.Address() */ string]NetworkClient
	connected  map[ /* node.Address() */ string]bool

	log logging.Logger
}

func NewValidatorConnectionManager(
	localNode *node.LocalNode,
	network Network,
	policy ballot.VotingThresholdPolicy,
	validators map[string]*node.Validator,
) ConnectionManager {
	return &ValidatorConnectionManager{
		localNode: localNode,

		network:    network,
		policy:     policy,
		validators: validators,

		clients:   map[string]NetworkClient{},
		connected: map[string]bool{},
		log:       log.New(logging.Ctx{"node": localNode.Alias()}),
	}
}

func (c *ValidatorConnectionManager) GetNodeAddress() string {
	return c.localNode.Address()
}

func (c *ValidatorConnectionManager) GetConnection(address string) (client NetworkClient) {
	c.Lock()
	defer c.Unlock()

	var ok bool
	client, ok = c.clients[address]
	if ok {
		return
	}

	var validator *node.Validator
	if validator, ok = c.validators[address]; !ok {
		return
	}

	client = c.network.GetClient(validator.Endpoint())
	if client != nil {
		c.clients[address] = client
	}

	return
}

func (c *ValidatorConnectionManager) Start() {
	c.log.Debug("starting to connect to validators", "validators", c.validators)
	for _, v := range c.validators {
		go c.connectingValidator(v)
	}
}

// setConnected returns `true` when the validator is newly connected or
// disconnected at first
func (c *ValidatorConnectionManager) setConnected(v *node.Validator, connected bool) bool {
	c.Lock()
	defer c.Unlock()

	old, found := c.connected[v.Address()]
	c.connected[v.Address()] = connected

	c.policy.SetConnected(c.countConnectedUnlocked())
	return !found || old != connected
}

func (c *ValidatorConnectionManager) AllConnected() []string {
	c.RLock()
	defer c.RUnlock()
	var connected []string
	for address, isConnected := range c.connected {
		if !isConnected {
			continue
		}
		connected = append(connected, address)
	}

	return connected
}

// Returns:
//   A list of all validators, including self
func (c *ValidatorConnectionManager) AllValidators() []string {
	var validators []string
	for address := range c.validators {
		validators = append(validators, address)
	}
	return append(validators, c.localNode.Address())
}

//
// Returns:
//   the number of validators which are currently connected
//
func (c *ValidatorConnectionManager) CountConnected() int {
	c.RLock()
	defer c.RUnlock()
	return c.countConnectedUnlocked()
}

func (c *ValidatorConnectionManager) countConnectedUnlocked() int {
	var count int
	for _, isConnected := range c.connected {
		if isConnected {
			count += 1
		}
	}
	return count
}

func (c *ValidatorConnectionManager) connectingValidator(v *node.Validator) {
	ticker := time.NewTicker(time.Second * 1)
	for _ = range ticker.C {
		err := c.connectValidator(v)

		if c.setConnected(v, err == nil) {
			if err == nil {
				c.log.Debug("validator is connected", "validator", v)
			} else {
				c.log.Debug("validator is disconnected", "validator", v, "error", err)
			}
		}
	}

	return
}

func (c *ValidatorConnectionManager) connectValidator(v *node.Validator) (err error) {
	client := c.GetConnection(v.Address())

	var b []byte
	b, err = client.Connect(c.localNode)
	if err != nil {
		return
	}

	// load and check validator info; addresses are same?
	var validator *node.Validator
	validator, err = node.NewValidatorFromString(b)
	if err != nil {
		return
	}
	if v.Address() != validator.Address() {
		err = errors.New("address is mismatch")
		return
	}

	return
}

func (c *ValidatorConnectionManager) ConnectionWatcher(t Network, conn net.Conn, state http.ConnState) {
	return
}

func (c *ValidatorConnectionManager) Broadcast(message common.Message) {
	c.RLock()
	defer c.RUnlock()
	for addr, connected := range c.connected {
		if connected {
			go func(v *node.Validator) {
				client := c.GetConnection(v.Address())

				var err error
				if message.GetType() == common.BallotMessage {
					_, err = client.SendBallot(message)
				} else if message.GetType() == string(common.TransactionMessage) {
					_, err = client.SendMessage(message)
				} else {
					panic("invalid message")
				}

				if err != nil {
					c.log.Error("failed to SendBallot", "error", err, "validator", v)
				}
			}(c.validators[addr])
		}
	}
	return
}
