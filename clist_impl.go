package simplelist

import (
	"sync"
	"sync/atomic"
	"unsafe"
)

type IntList struct {
	head   *intNode
	length int64
}

// getLength 获取数组长度
func (l *IntList) getLength() int {
	return int(atomic.LoadInt64(&l.length))
}

// incrLength 增长数组长度计数
func (l *IntList) incrLength() {
	atomic.AddInt64(&l.length, 1)
}

// decrLength 减少数组长度计数
func (l *IntList) decrLength() {
	atomic.AddInt64(&l.length, -1)
}

type intNode struct {
	value  int
	marked uint32
	mu     sync.Mutex
	next   *intNode
}

// isMarked 是否被标记
func (i *intNode) isMarked() bool {
	return atomic.LoadUint32(&i.marked) == 1
}

// setMarked 设置为「已标记」
func (i *intNode) setMarked() {
	atomic.StoreUint32(&i.marked, 1)
}

// getNext 读取下个节点
func (i *intNode) getNext() *intNode {
	return (*intNode)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&i.next))))
}

// getNext 设置下个节点
func (i *intNode) setNext(x *intNode) {
	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&i.next)), unsafe.Pointer(x))
}

func newIntNode(value int) *intNode {
	return &intNode{value: value}
}

// 返回一个全新的有序链表
func NewInt() ConcurrentList {
	return &IntList{head: newIntNode(0)}
}

func (l *IntList) Insert(value int) bool {
	var suc bool
	for !suc {
		a := l.head
		b := a.getNext()
		for b != nil && b.value < value {
			a = b
			b = b.getNext()
		}
		// 1、找到节点A和B，不存在就直接返回
		if b != nil && b.value == value {
			return false
		}
		// 尝试插入节点，若存在data race则进行重试
		suc = l.tryInsertNode(value, a, b)
	}
	return true
}

func (l *IntList) tryInsertNode(value int, a, b *intNode) bool {
	// 2、锁定节点a,检查a、b关系或a.marked
	a.mu.Lock()
	defer a.mu.Unlock() // 5、解锁节点A
	if a.next != b || a.isMarked() {
		return false  //插入失败
	}
	// 3、创建节点x
	x := newIntNode(value)
	// 4、插入节点，设置x.next = b, a.next = x
	x.setNext(b)
	a.setNext(x)
	l.incrLength()
	return true //插入成功
}

func (l *IntList) Delete(value int) bool {
	var suc bool
	for !suc {
		a := l.head
		b := a.getNext()
		// 1、找到节点a和b
		for b != nil && b.value < value {
			a = b
			b = b.getNext()
		}
		if b == nil || b.value != value {
			// 不存在则直接返回
			return false  //删除失败
		}
		suc = l.tryDeleteNode(a, b)
	}
	return true  //删除成功
}

func (l *IntList) tryDeleteNode(a, b *intNode) bool {
	// 2、锁定b
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.isMarked() {
		return false //解锁b
	}
	// 3、锁定a
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.next != b || a.isMarked() {
		return false //解锁a、b
	}
	// 4、设置b为marked，删除b
	b.setMarked()
	a.setNext(b.next)
	l.decrLength()
	return true //解锁a、b
}

func (l *IntList) Contains(value int) bool {
	// 1、找到节点x
	x := l.head.getNext()
	for x != nil && x.value < value {
		x = x.getNext()
	}
	if x == nil {
		return false
	}
	if x.value == value {
		// 存在则返回!isMarked
		return !x.isMarked()
	}
	// 不存在返回false
	return false
}

func (l *IntList) Range(f func(value int) bool) {
	x := l.head.getNext()
	for x != nil {
		if !f(x.value) {
			break
		}
		x = x.getNext()
	}
}

func (l *IntList) Len() int {
	return l.getLength()
}
