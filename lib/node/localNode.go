//
// Defines the `LocalNode` type of Node, which is our node
//
// A `LocalNode` is the local node, as opposed to a `Validator`
// which is the remote nodes this `LocalNode` sees.
//
// There should only be one `LocalNode` per program.
//
package node

import (
	"encoding/json"
	"fmt"
	"sync"

	"boscoin.io/sebak/lib/common"

	"github.com/stellar/go/keypair"
)

type LocalNode struct {
	sync.Mutex

	keypair *keypair.Full

	state           State
	alias           string
	bindEndpoint    *common.Endpoint
	publishEndpoint *common.Endpoint
	validators      map[ /* Node.Address() */ string]*Validator
}

func NewLocalNode(kp *keypair.Full, bindEndpoint *common.Endpoint, alias string) (n *LocalNode, err error) {
	if len(alias) < 1 {
		alias = MakeAlias(kp.Address())
	}

	n = &LocalNode{
		keypair:      kp,
		state:        StateNONE,
		alias:        alias,
		bindEndpoint: bindEndpoint,
		validators:   map[string]*Validator{},
	}

	return
}

func (n *LocalNode) String() string {
	return n.Alias()
}

func (n *LocalNode) Equal(a Node) bool {
	if n.Address() == a.Address() {
		return true
	}

	return false
}

func (n *LocalNode) State() State {
	return n.state
}

func (n *LocalNode) SetBooting() {
	n.state = StateBOOTING
}

func (n *LocalNode) SetSync() {
	n.state = StateSYNC
}

func (n *LocalNode) SetConsensus() {
	n.state = StateCONSENSUS
}

func (n *LocalNode) SetTerminating() {
	n.state = StateTERMINATING
}

func (n *LocalNode) Address() string {
	return n.keypair.Address()
}

func (n *LocalNode) Keypair() *keypair.Full {
	return n.keypair
}

func (n *LocalNode) Alias() string {
	return n.alias
}

func (n *LocalNode) Endpoint() *common.Endpoint {
	return n.bindEndpoint
}

func (n *LocalNode) BindEndpoint() *common.Endpoint {
	return n.bindEndpoint
}

func (n *LocalNode) PublishEndpoint() *common.Endpoint {
	return n.publishEndpoint
}

func (n *LocalNode) SetPublishEndpoint(endpoint *common.Endpoint) {
	n.publishEndpoint = endpoint
}

func (n *LocalNode) HasValidators(address string) bool {
	_, found := n.validators[address]
	return found
}

func (n *LocalNode) GetValidators() map[string]*Validator {
	return n.validators
}

func (n *LocalNode) AddValidators(validators ...*Validator) error {
	n.Lock()
	defer n.Unlock()

	for _, va := range validators {
		if n.Address() == va.Address() {
			continue
		}
		n.validators[va.Address()] = va
	}

	return nil
}

func (n *LocalNode) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"address":    n.Address(),
		"alias":      n.Alias(),
		"endpoint":   n.Endpoint().String(),
		"state":      n.State().String(),
		"validators": n.validators,
	})
}

func (n *LocalNode) Serialize() ([]byte, error) {
	return json.Marshal(n)
}

func (n *LocalNode) ConvertToValidator() *Validator {
	v, _ := NewValidator(n.Address(), n.Endpoint(), n.Alias())
	return v
}

func MakeAlias(address string) string {
	l := len(address)
	return fmt.Sprintf("%s.%s", address[:4], address[l-8:l-4])
}
