package main

type Buffer struct {
	buffer []string
	index  int
}

func (b *Buffer) Get() string {
	return b.buffer[b.index]
}

func (b *Buffer) GetByIdx(index int) string {
	return b.buffer[index]
}

func (b *Buffer) Clear() {
	if !b.Empty() {
		b.buffer = b.buffer[:0]
		b.index = 0
	}
}

func (b *Buffer) Size() int {
	return len(b.buffer)
}

func (b *Buffer) Empty() bool {
	if b.Size() == 0 {
		return true
	}
	return false
}

func (b *Buffer) Add(elem string) {
	b.buffer = append(b.buffer, elem)
	b.index = b.Size() - 1
}

func (b *Buffer) Append(elems []string) {
	b.buffer = elems
	b.index = b.Size() - 1
}

func (b *Buffer) Back() string {
	if b.Empty() {
		panic("Buffer is empty")
	}
	return b.buffer[b.Size()-1]
}

// Return next element in cycle
func (b *Buffer) Cycle() string {
	b.index = (b.index + 1) % b.Size()
	return b.buffer[b.index]
}
