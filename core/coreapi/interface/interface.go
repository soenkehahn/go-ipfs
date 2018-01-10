// Package iface defines IPFS Core API which is a set of interfaces used to
// interact with IPFS nodes.
package iface

import (
	"context"
	"errors"
	"io"
	"time"

	options "github.com/ipfs/go-ipfs/core/coreapi/interface/options"

	ipld "gx/ipfs/QmNwUEK7QbwSqyKBu3mMtToo8SUc6wQJ7gdZq4gGGJqfnf/go-ipld-format"
	cid "gx/ipfs/QmeSrf6pzut73u6zLQkRFQ3ygt3k6XFT2kjdYP8Tnkwwyg/go-cid"
)

// Path is a generic wrapper for paths used in the API. A path can be resolved
// to a CID using one of Resolve functions in the API.
type Path interface {
	// String returns the path as a string.
	String() string
	// Cid returns cid referred to by path
	Cid() *cid.Cid
	// Root returns cid of root path
	Root() *cid.Cid
	// Resolved returns whether path has been fully resolved
	Resolved() bool
}

// TODO: should we really copy these?
//       if we didn't, godoc would generate nice links straight to go-ipld-format
type Node ipld.Node
type Link ipld.Link

type Reader interface {
	io.ReadSeeker
	io.Closer
}

// IpnsEntry specifies the interface to IpnsEntries
type IpnsEntry interface {
	// Name returns IpnsEntry name
	Name() string
	// Value returns IpnsEntry value
	Value() Path
}

// Key specifies the interface to Keys in KeyAPI Keystore
type Key interface {
	// Key returns key name
	Name() string
	// Path returns key path
	Path() Path
}

// Pin holds information about pinned resource
type Pin interface {
	// Path to the pinned object
	Path() Path

	// Type of the pin
	Type() string
}

// PinStatus holds information about pin health
type PinStatus interface {
	// Ok indicates whether the pin has been verified to be correct
	Ok() bool

	// BadNodes returns any bad (usually missing) nodes from the pin
	BadNodes() []BadPinNode
}

// BadPinNode is a node that has been marked as bad by Pin.Verify
type BadPinNode interface {
	// Path is the path of the node
	Path() Path

	// Err is the reason why the node has been marked as bad
	Err() error
}

// CoreAPI defines an unified interface to IPFS for Go programs.
type CoreAPI interface {
	// Unixfs returns an implementation of Unixfs API.
	Unixfs() UnixfsAPI
	// Dag returns an implementation of Dag API.
	Dag() DagAPI
	// Name returns an implementation of Name API.
	Name() NameAPI
	// Key returns an implementation of Key API.
	Key() KeyAPI
	Pin() PinAPI

	// ResolvePath resolves the path using Unixfs resolver
	ResolvePath(context.Context, Path) (Path, error)

	// ResolveNode resolves the path (if not resolved already) using Unixfs
	// resolver, gets and returns the resolved Node
	ResolveNode(context.Context, Path) (Node, error)
}

// UnixfsAPI is the basic interface to immutable files in IPFS
type UnixfsAPI interface {
	// Add imports the data from the reader into merkledag file
	Add(context.Context, io.Reader) (Path, error)

	// Cat returns a reader for the file
	Cat(context.Context, Path) (Reader, error)

	// Ls returns the list of links in a directory
	Ls(context.Context, Path) ([]*Link, error)
}

// DagAPI specifies the interface to IPLD
type DagAPI interface {
	// Put inserts data using specified format and input encoding.
	// Unless used with WithCodec or WithHash, the defaults "dag-cbor" and
	// "sha256" are used.
	Put(ctx context.Context, src io.Reader, opts ...options.DagPutOption) (Path, error)

	// WithInputEnc is an option for Put which specifies the input encoding of the
	// data. Default is "json", most formats/codecs support "raw"
	WithInputEnc(enc string) options.DagPutOption

	// WithCodec is an option for Put which specifies the multicodec to use to
	// serialize the object. Default is cid.DagCBOR (0x71)
	WithCodec(codec uint64) options.DagPutOption

	// WithHash is an option for Put which specifies the multihash settings to use
	// when hashing the object. Default is based on the codec used
	// (mh.SHA2_256 (0x12) for DagCBOR). If mhLen is set to -1, default length for
	// the hash will be used
	WithHash(mhType uint64, mhLen int) options.DagPutOption

	// Get attempts to resolve and get the node specified by the path
	Get(ctx context.Context, path Path) (Node, error)

	// Tree returns list of paths within a node specified by the path.
	Tree(ctx context.Context, path Path, opts ...options.DagTreeOption) ([]Path, error)

	// WithDepth is an option for Tree which specifies maximum depth of the
	// returned tree. Default is -1 (no depth limit)
	WithDepth(depth int) options.DagTreeOption
}

// NameAPI specifies the interface to IPNS.
//
// IPNS is a PKI namespace, where names are the hashes of public keys, and the
// private key enables publishing new (signed) values. In both publish and
// resolve, the default name used is the node's own PeerID, which is the hash of
// its public key.
//
// You can use .Key API to list and generate more names and their respective keys.
type NameAPI interface {
	// Publish announces new IPNS name
	Publish(ctx context.Context, path Path, opts ...options.NamePublishOption) (IpnsEntry, error)

	// WithValidTime is an option for Publish which specifies for how long the
	// entry will remain valid. Default value is 24h
	WithValidTime(validTime time.Duration) options.NamePublishOption

	// WithKey is an option for Publish which specifies the key to use for
	// publishing. Default value is "self" which is the node's own PeerID.
	// The key parameter must be either PeerID or keystore key alias.
	//
	// You can use KeyAPI to list and generate more names and their respective keys.
	WithKey(key string) options.NamePublishOption

	// Resolve attempts to resolve the newest version of the specified name
	Resolve(ctx context.Context, name string, opts ...options.NameResolveOption) (Path, error)

	// WithRecursive is an option for Resolve which specifies whether to perform a
	// recursive lookup. Default value is false
	WithRecursive(recursive bool) options.NameResolveOption

	// WithLocal is an option for Resolve which specifies if the lookup should be
	// offline. Default value is false
	WithLocal(local bool) options.NameResolveOption

	// WithCache is an option for Resolve which specifies if cache should be used.
	// Default value is true
	WithCache(cache bool) options.NameResolveOption
}

// KeyAPI specifies the interface to Keystore
type KeyAPI interface {
	// Generate generates new key, stores it in the keystore under the specified
	// name and returns a base58 encoded multihash of it's public key
	Generate(ctx context.Context, name string, opts ...options.KeyGenerateOption) (Key, error)

	// WithType is an option for Generate which specifies which algorithm
	// should be used for the key. Default is options.RSAKey
	//
	// Supported key types:
	// * options.RSAKey
	// * options.Ed25519Key
	WithType(algorithm string) options.KeyGenerateOption

	// WithSize is an option for Generate which specifies the size of the key to
	// generated. Default is -1
	//
	// value of -1 means 'use default size for key type':
	//  * 2048 for RSA
	WithSize(size int) options.KeyGenerateOption

	// Rename renames oldName key to newName. Returns the key and whether another
	// key was overwritten, or an error
	Rename(ctx context.Context, oldName string, newName string, opts ...options.KeyRenameOption) (Key, bool, error)

	// WithForce is an option for Rename which specifies whether to allow to
	// replace existing keys.
	WithForce(force bool) options.KeyRenameOption

	// List lists keys stored in keystore
	List(ctx context.Context) ([]Key, error)

	// Remove removes keys from keystore. Returns ipns path of the removed key
	Remove(ctx context.Context, name string) (Path, error)
}

// type ObjectAPI interface {
// 	New() (cid.Cid, Object)
// 	Get(string) (Object, error)
// 	Links(string) ([]*Link, error)
// 	Data(string) (Reader, error)
// 	Stat(string) (ObjectStat, error)
// 	Put(Object) (cid.Cid, error)
// 	SetData(string, Reader) (cid.Cid, error)
// 	AppendData(string, Data) (cid.Cid, error)
// 	AddLink(string, string, string) (cid.Cid, error)
// 	RmLink(string, string) (cid.Cid, error)
// }

// type ObjectStat struct {
// 	Cid            cid.Cid
// 	NumLinks       int
// 	BlockSize      int
// 	LinksSize      int
// 	DataSize       int
// 	CumulativeSize int
// }

// PinAPI specifies the interface to pining
type PinAPI interface {
	// Add creates new pin, be default recursive - pinning the whole referenced
	// tree
	Add(context.Context, Path, ...options.PinAddOption) error

	// WithRecursive is an option for Add which specifies whether to pin an entire
	// object tree or just one object. Default: true
	WithRecursive(bool) options.PinAddOption

	// Ls returns list of pinned objects on this node
	Ls(context.Context, ...options.PinLsOption) ([]Pin, error)

	// WithType is an option for Ls which allows to specify which pin types should
	// be returned
	//
	// Supported values:
	// * "direct" - directly pinned objects
	// * "recursive" - roots of recursive pins
	// * "indirect" - indirectly pinned objects (referenced by recursively pinned
	//    objects)
	// * "all" - all pinned objects (default)
	WithType(string) options.PinLsOption

	// Rm removes pin for object specified by the path
	Rm(context.Context, Path) error

	// Update changes one pin to another, skipping checks for matching paths in
	// the old tree
	Update(ctx context.Context, from Path, to Path, opts ...options.PinUpdateOption) error

	// Verify verifies the integrity of pinned objects
	Verify(context.Context) (<-chan PinStatus, error)
}

var ErrIsDir = errors.New("object is a directory")
var ErrOffline = errors.New("can't resolve, ipfs node is offline")
