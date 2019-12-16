package instruction

import (
	"c0_compiler/internal/cc0_error"
	"c0_compiler/internal/common"
	"container/heap"
)

type Error = cc0_error.Error
type PriorityQueue = common.PriorityQueue

type FnInstructions struct {
	lines  *[]Line
	offset int
}

type Fn struct {
	instructions     *FnInstructions
	ReturnType       int
	Parameters       *[]string
	emptyMemorySlots *PriorityQueue
	stackSize        int
}

func InitFn(returnType int) (res *Fn) {
	res = &Fn{
		instructions:     &FnInstructions{lines: &[]Line{}, offset: 0},
		Parameters:       &[]string{},
		ReturnType:       returnType,
		emptyMemorySlots: &PriorityQueue{0},
	}
	heap.Init(res.emptyMemorySlots)
	return
}

func (f *Fn) GetLines() *[]Line {
	return f.instructions.lines
}

func (f *Fn) GetCurrentOffset() int {
	return len(*f.instructions.lines)
}

func (f *Fn) GetPreviousLine() *Line {
	lines := *f.instructions.lines
	return &(lines[len(lines)-2])
}

func (f *Fn) GetCurrentLine() *Line {
	lines := *f.instructions.lines
	return &(lines[len(lines)-1])
}

func (f *Fn) Append(instruction int, operands ...int) {
	i := GetInstruction(instruction)
	f.stackSize += i.changesToStackSize
	if !i.IsValidInstruction(operands...) {
		cc0_error.PrintfToStdErr("Incorrect usage of instruction 0x%x!\n", instruction)
		cc0_error.ThrowAndExit(cc0_error.Analyzer)
	}
	f.instructions.offset += i.offset
	*f.instructions.lines = append(*f.instructions.lines, Line{I: i, Operands: &operands})
}

func (f *Fn) NextMemorySlot() (slot int) {
	queue := f.emptyMemorySlots
	slot = queue.Pop().(int)
	if queue.Len() == 0 {
		queue.Push(slot + 1)
	}
	return
}

func (f *Fn) PopStack(reservedSize int) {
	f.Append(Popn, f.stackSize-reservedSize)
}
