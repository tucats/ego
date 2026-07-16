package ast

import "strings"

// Dump renders the tree rooted at node as an indented, human-readable outline,
// one node per line. Each line shows the node's String() description; children
// are indented beneath their parent. It is intended for debugging, test output,
// and the CLI "ego parse" command — not for source reconstruction.
func Dump(node Node) string {
	var b strings.Builder

	dump(&b, node, 0)

	return b.String()
}

func dump(b *strings.Builder, node Node, depth int) {
	if node == nil {
		return
	}

	b.WriteString(strings.Repeat("  ", depth))
	b.WriteString(node.String())

	if pos := node.Pos(); pos.IsValid() {
		b.WriteString("  @")
		b.WriteString(itoa(pos.Line))
		b.WriteString(":")
		b.WriteString(itoa(pos.Column))
	}

	b.WriteByte('\n')

	for _, child := range node.Children() {
		dump(b, child, depth+1)
	}
}
