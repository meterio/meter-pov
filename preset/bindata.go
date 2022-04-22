// Code generated by go-bindata.
// sources:
// mainnet/delegates.json
// shoal/delegates.json
// DO NOT EDIT!

package preset

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func bindataRead(data []byte, name string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("Read %q: %v", name, err)
	}

	var buf bytes.Buffer
	_, err = io.Copy(&buf, gz)
	clErr := gz.Close()

	if err != nil {
		return nil, fmt.Errorf("Read %q: %v", name, err)
	}
	if clErr != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

type asset struct {
	bytes []byte
	info  os.FileInfo
}

type bindataFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
}

func (fi bindataFileInfo) Name() string {
	return fi.name
}
func (fi bindataFileInfo) Size() int64 {
	return fi.size
}
func (fi bindataFileInfo) Mode() os.FileMode {
	return fi.mode
}
func (fi bindataFileInfo) ModTime() time.Time {
	return fi.modTime
}
func (fi bindataFileInfo) IsDir() bool {
	return false
}
func (fi bindataFileInfo) Sys() interface{} {
	return nil
}

var _mainnetDelegatesJson = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xac\xd5\x4b\xcf\xa2\x4a\x1a\x07\xf0\xfd\xfb\x29\x8c\x5b\x7b\xa4\x8a\x82\x82\x32\xe9\x05\x57\x01\x01\x51\xbc\x32\x99\x74\xb8\xcb\x1d\x01\xc5\xd7\x49\x7f\xf7\x89\x3d\xdd\x3b\x4f\x4e\xd2\xa7\xb7\xf5\xe4\xa9\xe4\xf9\xff\x16\xff\x7f\x7f\x4c\x26\xff\xfd\x98\x4c\x26\x93\x69\xed\x57\xf1\x74\x31\x99\x56\x7e\x56\xff\x0b\xc0\xe9\x97\xff\x3f\xfb\x51\xd4\xc5\x7d\xff\x9a\x80\x87\x48\x78\xa0\x26\x08\x0b\x84\x48\x40\x42\x22\x03\x78\x9a\x23\x0a\xa0\xd9\x88\x57\x68\x59\xe0\x62\x15\xb3\x10\xb0\xbf\x96\xdb\x5b\xf0\xad\x88\x3f\x5f\xcb\xa2\x12\xdd\x1b\xa3\xf7\x4c\x6f\xa4\xab\x6a\xe5\x14\x9e\xde\x9a\x1b\x24\x12\xca\x04\xa3\xdf\x8d\x7d\x55\xb9\x30\xb2\x35\xf7\x94\xc8\x0e\xb8\x8b\x0f\x6c\xb8\xe7\xbe\xba\x1d\xc7\x9c\x42\x7d\xc3\xe6\x54\x55\x93\x75\xb2\x33\x2c\x4b\xb1\xd6\x94\x39\x6c\x2b\xd7\x59\x37\x5f\x17\x8b\x85\x79\xcc\x4a\x5d\xcc\xda\x6e\x73\xbc\x11\x29\x59\x3f\xf6\x4b\x4b\x21\x25\x20\x84\x30\xfb\xe6\xb9\xf5\x6b\x94\x0d\x71\xb7\xa4\xe8\xe7\x75\xd8\x3f\x15\x5c\x8f\xb4\x73\xe7\xef\xad\xa0\x1a\xee\x66\xf7\x64\x92\xe0\x4a\x5f\x22\x36\xd2\xba\x8d\x9e\x1f\x4f\x2c\xc7\x24\x83\x20\x7c\xfd\x75\xc5\xbd\x19\xb2\x3a\xfd\xd6\x36\x63\xdc\x4d\x17\x13\x08\xc0\xcf\x41\x1d\x0f\x63\xd3\x15\xdf\x5e\x19\x4d\x17\x3f\x83\x9c\x4c\xa6\x59\xfb\xba\x18\xcd\xc1\x1c\x91\x39\x4f\xff\xfc\xe7\x95\x47\xd3\x0d\xd3\xc5\x84\xc7\x1c\xf8\xf1\xf4\xfd\x63\x32\xf9\xfe\xe5\xaf\x0c\xe8\xb7\x06\x88\x66\x31\xc3\x20\x49\xe5\x24\x4e\xc2\x21\x11\x55\x12\xf2\x1c\x93\xb0\x09\x8d\x08\x4d\xb3\x0a\x08\x10\xc3\x73\xca\x5b\x03\xdf\xdf\x76\x09\x33\x1a\xde\x65\x58\x96\x69\x95\x25\x89\xb1\x7c\xd4\x20\x0a\xb7\x5c\xa5\x07\x2e\x7b\xb5\x42\x59\xbc\x07\x89\xdc\x53\x75\x9e\x1f\x98\x3c\x72\xef\x59\x10\x6e\x22\x05\xfb\x69\x51\xa6\x71\xbd\xaf\xad\x5d\x29\x10\x0a\x7b\x11\xca\x06\xa8\x3d\x8e\xfa\xcb\xe0\x20\x72\x9a\xbb\xcd\x1a\xee\x6a\x91\xac\x68\xc5\x5e\xf6\x46\x66\xbf\xda\x1e\x8a\x87\x06\x73\xf7\x80\x67\xa2\x42\x86\x2b\xcc\x66\x66\xa9\x47\x1a\x95\x39\x68\x99\x70\xc5\xe8\x8a\xbb\x74\xf5\x70\x87\x32\x2d\x5d\x81\x48\x96\x7d\x6e\x19\x6b\x76\x8a\xed\x53\x00\x73\x41\xf9\xe7\x06\x70\x0e\x11\x9e\x43\x0c\x7e\x57\x01\xbd\x55\xe0\x69\x46\x15\x39\xc8\x05\x00\x8a\x98\x66\xfc\x10\x08\x41\x20\x07\x88\x85\x44\xc1\x04\x61\x00\x89\x0c\x25\x3a\x7e\xab\xe0\xad\xa9\x2b\xf2\x57\xaa\xab\x9f\x52\xfd\xc2\x02\x75\x65\xf3\xcf\xd2\xb6\x0a\x5e\x5a\x45\xd9\x39\xbe\xce\x3a\xdb\xac\x65\x5d\xdd\xb5\x16\x65\x9e\xaf\x42\x7a\xf3\xe5\x9b\xfd\x7c\x68\xbb\x54\x3d\x65\x22\x95\x55\x1b\x33\x27\x5e\xa9\x6e\xac\x15\x9b\x53\x52\x4e\x1c\xe5\xa5\xe0\x0c\xb7\x07\xea\x96\xba\x0f\xf5\x3e\x19\xb9\xfd\xe1\x93\x9d\x99\x85\xfa\x3c\xeb\x8c\xaa\x1e\x10\xea\xfb\x6b\xb5\xc6\xd7\x9d\xcc\x7b\x71\x13\x7a\x9f\x81\x6d\x8c\x2b\xc7\xd4\xdb\x54\xba\x75\x36\x3d\xe4\x1b\x79\x5c\x3a\xf0\xbc\x1a\xe0\xc9\x63\x96\xa6\x55\xc2\x75\x37\xfe\x11\x05\x9a\x41\x73\x88\xb9\xdf\x55\x60\xde\x2a\x28\x18\xf3\x04\x41\x86\x13\x38\x04\xa2\x48\x48\x48\x0c\xfc\x98\xa1\x7d\x99\x8b\x25\x9e\x61\xb1\x0c\xd5\x90\x05\xf2\x3b\x05\xa3\xb2\x5c\x55\x3c\xd4\x1d\x30\xdc\x5d\x83\xad\x8b\xd1\xb1\xbb\x63\x79\xf0\x57\x2d\xcf\x0f\xba\x77\x19\xfb\x9a\x47\x97\x7a\x1c\x05\x1f\xd2\xc7\x95\xd1\xa5\x86\x79\x4e\xab\x53\x8f\xc2\x5b\xe4\x35\x8a\x2d\x5f\xae\xed\xe1\x68\xee\x1a\x52\x99\x4d\x1a\xb2\x89\x77\x67\x5e\x0a\x7a\x33\x5b\x93\x6e\xd0\x35\x6b\x2b\xce\xe8\x00\xac\x84\x5a\xd8\x9e\x77\x31\xd7\xca\xb3\x0e\xed\x91\xdc\x1d\x5c\x12\x39\x4a\xab\xa5\xab\xd0\x3c\xb9\x6d\x54\x17\xc5\x83\x6f\x83\x24\x12\xa5\x27\xe6\xfb\x59\x3f\x14\xc7\xd0\xcf\xb7\x92\xa2\xd9\x60\x0d\x87\x40\x62\xd2\x3f\xa0\x40\x23\x34\x67\xe7\x0c\xfe\x5d\x04\xf6\x2d\x02\x01\x48\x65\x39\x4e\xc2\x80\x23\x58\xf4\x43\x3a\xf0\x03\x9a\x0f\x39\x28\xc3\x50\x84\x89\x2c\x61\x89\x09\x19\x40\xde\x21\x88\x4f\xfd\x33\x53\x2a\xad\xbc\xcc\x0a\x72\x0d\x6c\x2a\xdf\x1c\xab\x23\x27\x25\xea\x33\xaf\x6a\xdb\x70\x06\xf7\x20\xb5\x4e\x13\x68\x33\x50\x50\xf4\x33\x1d\x9c\x2c\x64\xf6\xa1\x26\x70\xc7\x6c\xdd\x45\xc7\x2d\xf6\x85\xbd\x65\x52\xf1\xe1\xee\x68\x54\xcb\x98\x5b\xfa\xf1\xa3\x14\x9c\xfb\xcd\x7e\xec\x02\x56\x50\x35\x61\x53\x79\xda\x25\x31\xee\xa8\x74\x41\x06\xa3\x56\x5f\xd9\x87\xdb\xe7\xfe\xe6\x1b\xee\x53\x95\x86\x62\x49\x79\x97\x88\xd8\x60\x2f\x5c\x62\xa2\xd3\x10\x2e\x37\x26\x20\x59\x5c\xca\xd0\xd2\x04\xe1\x1e\xdd\xaf\xfb\x2b\xa5\x3d\xea\xf4\x0f\x94\x02\x66\xe7\xec\x8b\xe2\xef\x6a\xe1\xe3\x3f\x1f\xff\x0b\x00\x00\xff\xff\xd4\x6a\xd9\xf2\x9e\x07\x00\x00")

func mainnetDelegatesJsonBytes() ([]byte, error) {
	return bindataRead(
		_mainnetDelegatesJson,
		"mainnet/delegates.json",
	)
}

func mainnetDelegatesJson() (*asset, error) {
	bytes, err := mainnetDelegatesJsonBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "mainnet/delegates.json", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _shoalDelegatesJson = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xac\xd1\x49\x8f\xa3\x46\x1c\x05\xf0\x7b\x7f\x0a\xe4\x2b\x23\xbb\xa8\x2a\x6a\xb1\x34\x07\x30\x18\x1b\xbc\xd1\xde\x89\xa2\x16\xab\x6d\x08\x66\x71\x01\x6e\x47\xf3\xdd\x23\x4f\x7a\x6e\x96\x22\x25\xb9\xbe\xd2\xbf\xa4\xf7\x7e\xbf\xbd\x49\xd2\x9f\x6f\x92\x24\x49\xbd\xab\x9f\xc7\xbd\xa1\xd4\x13\x4a\xef\xdb\xdf\x89\x1f\x45\x75\x7c\xbb\x3d\x43\x70\x27\x2a\xd3\x49\xe4\x13\x0a\x11\xa7\x3c\x1e\xa9\x24\x89\x13\x05\x72\x32\x56\x14\x15\x53\x8a\x09\xd0\x18\xe5\xf4\xd7\x71\xd9\x04\x1f\x59\xfc\xf9\x3c\xd6\x0d\x56\x7b\x5d\x1a\x5d\x6a\x7e\xbe\x44\xa2\xba\x1b\x8e\x5a\x07\x81\x55\x5b\x6b\x8f\xeb\x1e\x78\xe7\x2c\xd5\x44\x79\x77\xa2\x99\x0d\xae\x48\x0f\xc6\xf1\x14\xa8\x22\x28\xc5\x5e\xb4\xb6\x7a\xc8\xb3\x1c\xab\xa1\xd3\x95\x96\x57\xdd\x11\xf7\x41\x33\xcf\x7c\x0e\xbe\x0f\x87\xc3\x84\x37\xe7\xb5\x15\x6e\x0e\xef\x7a\xb6\xac\xca\x63\x63\x64\x69\x87\xb3\x3c\x6c\x73\xcd\x9c\x59\x7e\x43\xe7\xf6\xce\x3a\xb9\xdd\x4a\xb6\x08\x92\xdb\x47\x67\x6e\xe5\xfd\xa8\x01\x31\x58\x8c\xf5\xdc\x1e\x97\xb7\x0d\xf7\xdc\xd3\xb4\xf5\xcb\x47\x79\x54\xa2\xf6\xe2\x8c\x5d\xed\xfb\xaf\x16\x6d\x21\x2e\xd7\xd3\x47\x59\x74\x71\xdd\x1b\x4a\x0a\x00\x5f\x0f\xd7\x58\x74\x45\x9d\x7d\x3c\x37\xea\x0d\xbf\x36\x94\xa4\xde\xa5\x7c\x36\x46\x6a\x9f\x29\x7d\xb5\xaf\x30\xf4\xf5\xd3\x73\x91\xa2\x16\xbd\xa1\xc4\x08\x05\x3f\xa3\x1f\x6f\x92\xf4\xe3\xdb\x0b\x00\xf8\x12\x00\x61\xdd\xe0\x14\x82\x04\x47\x0c\x19\x01\x0c\x59\x44\xa3\x98\xd1\x78\x84\x58\x40\x19\x82\x08\x28\x23\x8d\xbc\x04\x98\x54\x83\x45\x1e\x06\xf1\x72\x3d\xe8\xcc\xca\xb3\xac\xe3\xf2\x94\x53\x3e\x10\xb3\xcc\xfb\x04\x00\x4f\xc7\x0d\x3e\xa5\x6b\x41\xd3\x8b\x31\x48\x66\xb5\x7d\x8b\x9c\xf5\xa4\x8a\x07\x85\x5b\x6d\xd4\x4f\xca\x84\x20\x13\xf4\xa8\x81\x42\xce\xed\x41\xc4\x83\x85\xe8\xe8\x4f\x80\x60\x51\x76\xc9\x3a\xb5\x83\x5b\xb6\x55\xc2\x87\x9e\x82\x72\x00\x8f\xf8\x06\x50\x3a\x61\xf9\x9d\x5f\x77\xb2\x7f\x3e\xa8\x2a\x53\xd4\xc9\xe2\xdd\x02\x48\xbe\x9a\xe9\x72\x96\x9d\xf1\x3e\x5c\x9c\xb6\x31\x9c\xbb\xce\x6e\xd3\x31\x94\x9f\x16\x5b\x66\x37\x94\xcf\xc7\xec\x64\xfe\x57\x00\x8c\xfb\x10\x92\x3e\x44\x7d\xca\xff\x85\x00\x7a\x29\x10\x93\x90\x22\x85\x6a\x90\x28\x26\xc0\x2a\xf2\x09\xe5\x68\x0c\x42\x13\x62\x42\x42\x15\xb0\x08\x25\x30\x04\xf8\x95\x80\x76\x6c\x5c\x39\x0a\xcb\x89\xb7\xa2\xc9\x45\x6f\x9c\x04\x44\x91\xe0\x55\x65\x06\xf7\x38\x1b\xa8\xc6\xd8\x38\x15\x8c\xd3\xf0\x1d\x4f\x8a\x9d\x71\xf8\xa3\x9e\x08\xbb\x5d\x45\x23\x72\x1c\xe9\x73\xbc\x0d\xd7\x7a\xc5\x65\x43\xb7\x38\x9f\xde\x36\xa6\x7b\x4d\xd6\x9e\x6f\x3e\x05\x56\xce\xbe\xc2\x3b\xb7\x6a\x0f\x44\xd6\x65\x64\xd5\xed\xf2\xba\x36\x67\xc5\x4a\x40\xed\xec\x24\x8b\xe2\xe1\x1f\xf1\x65\xfa\x40\x02\x1a\x6d\x11\x0b\x27\x32\x28\xa8\x57\xc0\xed\x36\x8c\xec\xaf\xa1\xa3\xdc\xb6\x1a\x05\x73\x3b\x5d\xda\x03\x99\x70\x4f\x73\x65\xed\xff\x11\x40\xa0\x0f\x21\xec\x43\x55\xf9\x07\x83\xb7\xdf\xdf\xfe\x0a\x00\x00\xff\xff\x30\xa9\xf0\x14\x8a\x04\x00\x00")

func shoalDelegatesJsonBytes() ([]byte, error) {
	return bindataRead(
		_shoalDelegatesJson,
		"shoal/delegates.json",
	)
}

func shoalDelegatesJson() (*asset, error) {
	bytes, err := shoalDelegatesJsonBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "shoal/delegates.json", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

// Asset loads and returns the asset for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func Asset(name string) ([]byte, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("Asset %s can't read by error: %v", name, err)
		}
		return a.bytes, nil
	}
	return nil, fmt.Errorf("Asset %s not found", name)
}

// MustAsset is like Asset but panics when Asset would return an error.
// It simplifies safe initialization of global variables.
func MustAsset(name string) []byte {
	a, err := Asset(name)
	if err != nil {
		panic("asset: Asset(" + name + "): " + err.Error())
	}

	return a
}

// AssetInfo loads and returns the asset info for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func AssetInfo(name string) (os.FileInfo, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("AssetInfo %s can't read by error: %v", name, err)
		}
		return a.info, nil
	}
	return nil, fmt.Errorf("AssetInfo %s not found", name)
}

// AssetNames returns the names of the assets.
func AssetNames() []string {
	names := make([]string, 0, len(_bindata))
	for name := range _bindata {
		names = append(names, name)
	}
	return names
}

// _bindata is a table, holding each asset generator, mapped to its name.
var _bindata = map[string]func() (*asset, error){
	"mainnet/delegates.json": mainnetDelegatesJson,
	"shoal/delegates.json": shoalDelegatesJson,
}

// AssetDir returns the file names below a certain
// directory embedded in the file by go-bindata.
// For example if you run go-bindata on data/... and data contains the
// following hierarchy:
//     data/
//       foo.txt
//       img/
//         a.png
//         b.png
// then AssetDir("data") would return []string{"foo.txt", "img"}
// AssetDir("data/img") would return []string{"a.png", "b.png"}
// AssetDir("foo.txt") and AssetDir("notexist") would return an error
// AssetDir("") will return []string{"data"}.
func AssetDir(name string) ([]string, error) {
	node := _bintree
	if len(name) != 0 {
		cannonicalName := strings.Replace(name, "\\", "/", -1)
		pathList := strings.Split(cannonicalName, "/")
		for _, p := range pathList {
			node = node.Children[p]
			if node == nil {
				return nil, fmt.Errorf("Asset %s not found", name)
			}
		}
	}
	if node.Func != nil {
		return nil, fmt.Errorf("Asset %s not found", name)
	}
	rv := make([]string, 0, len(node.Children))
	for childName := range node.Children {
		rv = append(rv, childName)
	}
	return rv, nil
}

type bintree struct {
	Func     func() (*asset, error)
	Children map[string]*bintree
}
var _bintree = &bintree{nil, map[string]*bintree{
	"mainnet": &bintree{nil, map[string]*bintree{
		"delegates.json": &bintree{mainnetDelegatesJson, map[string]*bintree{}},
	}},
	"shoal": &bintree{nil, map[string]*bintree{
		"delegates.json": &bintree{shoalDelegatesJson, map[string]*bintree{}},
	}},
}}

// RestoreAsset restores an asset under the given directory
func RestoreAsset(dir, name string) error {
	data, err := Asset(name)
	if err != nil {
		return err
	}
	info, err := AssetInfo(name)
	if err != nil {
		return err
	}
	err = os.MkdirAll(_filePath(dir, filepath.Dir(name)), os.FileMode(0755))
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(_filePath(dir, name), data, info.Mode())
	if err != nil {
		return err
	}
	err = os.Chtimes(_filePath(dir, name), info.ModTime(), info.ModTime())
	if err != nil {
		return err
	}
	return nil
}

// RestoreAssets restores an asset under the given directory recursively
func RestoreAssets(dir, name string) error {
	children, err := AssetDir(name)
	// File
	if err != nil {
		return RestoreAsset(dir, name)
	}
	// Dir
	for _, child := range children {
		err = RestoreAssets(dir, filepath.Join(name, child))
		if err != nil {
			return err
		}
	}
	return nil
}

func _filePath(dir, name string) string {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	return filepath.Join(append([]string{dir}, strings.Split(cannonicalName, "/")...)...)
}

