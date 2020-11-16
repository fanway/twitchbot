package buffer

type Buffer struct {
	Buffer []string
	Index  int
}

func (b *Buffer) Get() string {
	return b.Buffer[b.Index]
}

func (b *Buffer) GetByIdx(index int) string {
	return b.Buffer[index]
}

func (b *Buffer) Clear() {
	if !b.Empty() {
		b.Buffer = b.Buffer[:0]
		b.Index = 0
	}
}

func (b *Buffer) Size() int {
	return len(b.Buffer)
}

func (b *Buffer) Empty() bool {
	if b.Size() == 0 {
		return true
	}
	return false
}

func (b *Buffer) Add(elem string) {
	b.Buffer = append(b.Buffer, elem)
	b.Index = b.Size() - 1
}

func (b *Buffer) Append(elems []string) {
	b.Buffer = elems
	b.Index = b.Size() - 1
}

func (b *Buffer) Back() string {
	if b.Empty() {
		panic("Buffer is empty")
	}
	return b.Buffer[b.Size()-1]
}

// Return next element in cycle
func (b *Buffer) Cycle() string {
	b.Index = (b.Index + 1) % b.Size()
	return b.Buffer[b.Index]
}
