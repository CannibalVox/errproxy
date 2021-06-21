package multiptr

type Struct struct{}

func (s Struct) ValueOut() (string, error) {
	return "value", nil
}

type StructMultiPtr struct{}

func (s StructMultiPtr) ValueOut() (string, error) {
	return "value", nil
}

func (s *StructMultiPtr) PtrOut() (string, error) {
	return "ptr", nil
}

type StructPtr struct{}

func (s *StructPtr) PtrOut() (string, error) {
	return "ptr", nil
}

type Interface interface {
	IFaceOut() (string, error)
}

type UseAll struct{}

func (u *UseAll) Struct() Struct {
	return Struct{}
}

func (u *UseAll) AcceptStruct(s Struct) error {
	return nil
}

func (u *UseAll) StructPtr() *StructPtr {
	return &StructPtr{}
}

func (u *UseAll) AcceptStructPtr(s *StructPtr) error {
	return nil
}

func (u *UseAll) Interface() Interface {
	return nil
}

func (u *UseAll) AcceptInterface(i Interface) error {
	return nil
}

func (u *UseAll) StructMultiPtrValue() StructMultiPtr {
	return StructMultiPtr{}
}

func (u *UseAll) AcceptStructMultiPtrValue(s StructMultiPtr) error {
	return nil
}

func (u *UseAll) StructMultiPtrPtr() *StructMultiPtr {
	return &StructMultiPtr{}
}

func (u *UseAll) AcceptStructMultiPtrPtr(s *StructMultiPtr) error {
	return nil
}
