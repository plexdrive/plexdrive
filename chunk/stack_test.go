package chunk

import "testing"

func TestOOB(t *testing.T) {
	stack := NewStack(1)

	stack.Push(1)
	stack.Touch(1)
}

func TestAddToStack(t *testing.T) {
	stack := NewStack(1)

	stack.Push(1)
	stack.Push(2)
	stack.Push(3)
	stack.Push(4)

	stack.Touch(1)
	stack.Touch(3)

	stack.Purge(4)

	v := stack.Pop()
	if 4 != v {
		t.Fatalf("Expected 4 got %v", v)
	}

	v = stack.Pop()
	if 2 != v {
		t.Fatalf("Expected 2 got %v", v)
	}

	v = stack.Pop()
	if 1 != v {
		t.Fatalf("Expected 1 got %v", v)
	}

	v = stack.Pop()
	if 3 != v {
		t.Fatalf("Expected 3 got %v", v)
	}

	v = stack.Pop()
	if -1 != v {
		t.Fatalf("Expected -1 got %v", v)
	}
}

func TestLen(t *testing.T) {
	stack := NewStack(1)

	v := stack.Len()
	if 0 != v {
		t.Fatalf("Expected 0 got %v", v)
	}

	stack.Push(1)
	v = stack.Len()
	if 1 != v {
		t.Fatalf("Expected 1 got %v", v)
	}

	_ = stack.Pop()
	v = stack.Len()
	if 0 != v {
		t.Fatalf("Expected 0 got %v", v)
	}

}
