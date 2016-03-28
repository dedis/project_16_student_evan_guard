package randhound

import (
	"bytes"
	"encoding/binary"
	"time"

	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/random"
)

func (rh *RandHound) chooseTrustees(Rc, Rs []byte) (map[int]int, []abstract.Point) {

	// Seed PRNG for selection of trustees
	var seed []byte
	seed = append(seed, Rc...)
	seed = append(seed, Rs...)
	prng := rh.Node.Suite().Cipher(seed)

	// Choose trustees uniquely
	shareIdx := make(map[int]int)
	trustees := make([]abstract.Point, rh.Group.K)
	tns := rh.Tree().ListNodes()
	j := 0
	for len(shareIdx) < rh.Group.K {
		i := int(random.Uint64(prng) % uint64(len(tns)))
		// Add trustee only if not done so before; choosing yourself as an trustee is fine; ignore leader at index 0
		if _, ok := shareIdx[i]; !ok && !tns[i].IsRoot() {
			shareIdx[i] = j // j is the share index
			trustees[j] = tns[i].Entity.Public
			j += 1
		}
	}
	return shareIdx, trustees
}

func (rh *RandHound) hash(bytes ...[]byte) []byte {
	return abstract.Sum(rh.Node.Suite(), bytes...)
}

func (rh *RandHound) newGroup(nodes int, trustees int) (*Group, []byte, error) {

	n := nodes    // Number of nodes (peers + leader)
	k := trustees // Number of trustees (= shares generaetd per peer)
	buf := new(bytes.Buffer)

	// Setup group parameters: note that T <= R <= K must hold;
	// T = R for simplicity, might change later
	gp := [6]int{
		n,           // N: total number of nodes (peers + leader)
		n / 3,       // F: maximum number of Byzantine nodes tolerated
		n - (n / 3), // L: minimum number of non-Byzantine nodes required
		k,           // K: total number of trustees (= shares generated per peer)
		(k + 1) / 2, // R: minimum number of signatures needed to certify a deal
		(k + 1) / 2, // T: minimum number of shares needed to reconstruct a secret
	}

	// Include public keys of all nodes into group ID
	for _, x := range rh.Tree().ListNodes() {
		pub, err := x.Entity.Public.MarshalBinary()
		if err != nil {
			return nil, nil, err
		}
		err = binary.Write(buf, binary.LittleEndian, pub)
		if err != nil {
			return nil, nil, err
		}
	}

	// Include group parameters into group ID
	for _, g := range gp {
		err := binary.Write(buf, binary.LittleEndian, uint32(g))
		if err != nil {
			return nil, nil, err
		}
	}

	return &Group{
		N: gp[0],
		F: gp[1],
		L: gp[2],
		K: gp[3],
		R: gp[4],
		T: gp[5]}, rh.hash(buf.Bytes()), nil
}

func (rh *RandHound) newSession(public abstract.Point, purpose string, time time.Time) (*Session, []byte, error) {

	pub, err := public.MarshalBinary()
	if err != nil {
		return nil, nil, err
	}

	tm, err := time.MarshalBinary()
	if err != nil {
		return nil, nil, err
	}

	return &Session{
		Fingerprint: pub,
		Purpose:     purpose,
		Time:        time}, rh.hash(pub, []byte(purpose), tm), nil
}

func (rh *RandHound) nodeIdx() int {
	return rh.Node.TreeNode().EntityIdx
}

func (rh *RandHound) sendToChildren(msg interface{}) error {
	for _, c := range rh.Children() {
		err := rh.SendTo(c, msg)
		if err != nil {
			return err
		}
	}
	return nil
}
