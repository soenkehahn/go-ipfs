package republisher

import (
	"context"
	"errors"
	"time"

	keystore "github.com/ipfs/go-ipfs/keystore"
	namesys "github.com/ipfs/go-ipfs/namesys"
	pb "github.com/ipfs/go-ipfs/namesys/pb"
	path "github.com/ipfs/go-ipfs/path"
	dshelp "github.com/ipfs/go-ipfs/thirdparty/ds-help"

	routing "gx/ipfs/QmPCGUjMRuBcPybZFpjhzpifwPP9wPRoiy5geTQKU4vqWA/go-libp2p-routing"
	goprocess "gx/ipfs/QmSF8fPo3jgVBAy8fpdjjYqgG87dkJgUprRBHRd2tmfgpP/goprocess"
	gpctx "gx/ipfs/QmSF8fPo3jgVBAy8fpdjjYqgG87dkJgUprRBHRd2tmfgpP/goprocess/context"
	logging "gx/ipfs/QmSpJByNKFX1sCsHBEp3R73FL4NF6FnQTEGyNAXHm2GS52/go-log"
	recpb "gx/ipfs/QmWGtsyPYEoiqTtWLpeUA2jpW4YSZgarKDD2zivYAFz7sR/go-libp2p-record/pb"
	peer "gx/ipfs/QmWNY7dV54ZDYmTA1ykVdwNCqC11mpU4zSUp6XDpLTH9eG/go-libp2p-peer"
	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
	ic "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
	ds "gx/ipfs/QmdHG8MAuARdGHxx4rPQASLcvhz24fzjSQq7AJRAQEorq5/go-datastore"
)

var errNoEntry = errors.New("no previous entry")

var log = logging.Logger("ipns-repub")

// DefaultRebroadcastInterval is the default interval at which we rebroadcast IPNS records
var DefaultRebroadcastInterval = time.Hour * 4

// InitialRebroadcastDelay is the delay before first broadcasting IPNS records on start
var InitialRebroadcastDelay = time.Minute * 1

// FailureRetryInterval is the interval at which we retry IPNS records broadcasts (when they fail)
var FailureRetryInterval = time.Minute * 5

// DefaultRecordLifetime is the default lifetime for IPNS records
const DefaultRecordLifetime = time.Hour * 24

type Republisher struct {
	r    routing.ValueStore
	ds   ds.Datastore
	self ic.PrivKey
	ks   keystore.Keystore

	Interval time.Duration

	// how long records that are republished should be valid for
	RecordLifetime time.Duration
}

// NewRepublisher creates a new Republisher
func NewRepublisher(r routing.ValueStore, ds ds.Datastore, self ic.PrivKey, ks keystore.Keystore) *Republisher {
	return &Republisher{
		r:              r,
		ds:             ds,
		self:           self,
		ks:             ks,
		Interval:       DefaultRebroadcastInterval,
		RecordLifetime: DefaultRecordLifetime,
	}
}

func (rp *Republisher) Run(proc goprocess.Process) {
	timer := time.NewTimer(InitialRebroadcastDelay)
	defer timer.Stop()
	if rp.Interval < InitialRebroadcastDelay {
		timer.Reset(rp.Interval)
	}

	for {
		select {
		case <-timer.C:
			timer.Reset(rp.Interval)
			err := rp.republishEntries(proc)
			if err != nil {
				log.Error("Republisher failed to republish: ", err)
				if FailureRetryInterval < rp.Interval {
					timer.Reset(FailureRetryInterval)
				}
			}
		case <-proc.Closing():
			return
		}
	}
}

func (rp *Republisher) republishEntries(p goprocess.Process) error {
	ctx, cancel := context.WithCancel(gpctx.OnClosingContext(p))
	defer cancel()

	err := rp.republishEntry(ctx, rp.self)
	if err != nil {
		return err
	}

	if rp.ks != nil {
		keyNames, err := rp.ks.List()
		if err != nil {
			return err
		}
		for _, name := range keyNames {
			priv, err := rp.ks.Get(name)
			if err != nil {
				return err
			}
			err = rp.republishEntry(ctx, priv)
			if err != nil {
				return err
			}

		}
	}

	return nil
}

func (rp *Republisher) republishEntry(ctx context.Context, priv ic.PrivKey) error {
	id, err := peer.IDFromPrivateKey(priv)
	if err != nil {
		return err
	}

	log.Debugf("republishing ipns entry for %s", id)

	// Look for it locally only
	_, ipnskey := namesys.IpnsKeysForID(id)
	p, seq, err := rp.getLastVal(ipnskey)
	if err != nil {
		if err == errNoEntry {
			return nil
		}
		return err
	}

	// update record with same sequence number
	eol := time.Now().Add(rp.RecordLifetime)
	err = namesys.PutRecordToRouting(ctx, priv, p, seq, eol, rp.r, id)
	if err != nil {
		println("put record to routing error: " + err.Error())
		return err
	}

	return nil
}

func (rp *Republisher) getLastVal(k string) (path.Path, uint64, error) {
	ival, err := rp.ds.Get(dshelp.NewKeyFromBinary([]byte(k)))
	if err != nil {
		// not found means we dont have a previously published entry
		return "", 0, errNoEntry
	}

	val := ival.([]byte)
	dhtrec := new(recpb.Record)
	err = proto.Unmarshal(val, dhtrec)
	if err != nil {
		return "", 0, err
	}

	// extract published data from record
	e := new(pb.IpnsEntry)
	err = proto.Unmarshal(dhtrec.GetValue(), e)
	if err != nil {
		return "", 0, err
	}
	return path.Path(e.Value), e.GetSequence(), nil
}
