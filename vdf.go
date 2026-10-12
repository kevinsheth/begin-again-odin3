package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
)

// Steam's binary VDF, as used by shortcuts.vdf. Values keep their type and
// order so an untouched file is written back byte for byte.
const (
	vdfMap    = 0x00
	vdfString = 0x01
	vdfInt    = 0x02
	vdfUint64 = 0x07
	vdfEnd    = 0x08
)

type vdfNode struct {
	kind  byte
	key   string
	str   string
	num   uint64
	nodes []*vdfNode
}

func (n *vdfNode) child(key string) *vdfNode {
	for _, c := range n.nodes {
		if strings.EqualFold(c.key, key) {
			return c
		}
	}
	return nil
}

func (n *vdfNode) setString(key, value string) {
	if c := n.child(key); c != nil {
		c.kind, c.str = vdfString, value
		return
	}
	n.nodes = append(n.nodes, &vdfNode{kind: vdfString, key: key, str: value})
}

func (n *vdfNode) setInt(key string, value uint32) {
	if c := n.child(key); c != nil {
		c.kind, c.num = vdfInt, uint64(value)
		return
	}
	n.nodes = append(n.nodes, &vdfNode{kind: vdfInt, key: key, num: uint64(value)})
}

func parseVDF(data []byte) (*vdfNode, error) {
	root := &vdfNode{kind: vdfMap}
	rest, err := parseVDFMap(data, root)
	if err != nil {
		return nil, err
	}
	if len(rest) != 0 {
		return nil, fmt.Errorf("%d unexpected bytes after the end", len(rest))
	}
	return root, nil
}

func parseVDFMap(data []byte, parent *vdfNode) ([]byte, error) {
	for {
		if len(data) == 0 {
			return nil, fmt.Errorf("file ends inside %q", parent.key)
		}
		kind := data[0]
		data = data[1:]
		if kind == vdfEnd {
			return data, nil
		}
		key, rest, err := vdfCString(data)
		if err != nil {
			return nil, err
		}
		data = rest
		node := &vdfNode{kind: kind, key: key}
		switch kind {
		case vdfMap:
			if data, err = parseVDFMap(data, node); err != nil {
				return nil, err
			}
		case vdfString:
			if node.str, data, err = vdfCString(data); err != nil {
				return nil, err
			}
		case vdfInt:
			if len(data) < 4 {
				return nil, fmt.Errorf("truncated number %q", key)
			}
			node.num, data = uint64(binary.LittleEndian.Uint32(data)), data[4:]
		case vdfUint64:
			if len(data) < 8 {
				return nil, fmt.Errorf("truncated number %q", key)
			}
			node.num, data = binary.LittleEndian.Uint64(data), data[8:]
		default:
			return nil, fmt.Errorf("unknown value type %#x for %q", kind, key)
		}
		parent.nodes = append(parent.nodes, node)
	}
}

func vdfCString(data []byte) (string, []byte, error) {
	end := bytes.IndexByte(data, 0)
	if end < 0 {
		return "", nil, fmt.Errorf("unterminated string")
	}
	return string(data[:end]), data[end+1:], nil
}

func (n *vdfNode) encode() []byte {
	var buf bytes.Buffer
	encodeVDFMap(&buf, n)
	return buf.Bytes()
}

func encodeVDFMap(buf *bytes.Buffer, n *vdfNode) {
	for _, c := range n.nodes {
		buf.WriteByte(c.kind)
		buf.WriteString(c.key)
		buf.WriteByte(0)
		switch c.kind {
		case vdfMap:
			encodeVDFMap(buf, c)
		case vdfString:
			buf.WriteString(c.str)
			buf.WriteByte(0)
		case vdfInt:
			buf.Write(binary.LittleEndian.AppendUint32(nil, uint32(c.num)))
		case vdfUint64:
			buf.Write(binary.LittleEndian.AppendUint64(nil, c.num))
		}
	}
	buf.WriteByte(vdfEnd)
}
