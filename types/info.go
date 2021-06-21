package types

import (
	"fmt"
	gotypes "go/types"
	"strings"
)

type TypeMode int

const (
	TypeStruct TypeMode = TypeMode(iota)
	TypeInterface
	TypeOther
)

type WrapStatus int

const (
	WrapStatusDont WrapStatus = WrapStatus(iota) // Do not wrap this type
	WrapStatusSoft                               // This type is an interface- a new implementation should be written that wraps the interface
	WrapStatusHard                               // An entirely new type needs to be written that wraps the old type
	// Interfaces can be "soft" or "hard", other types can only be "hard".  Interfaces are hard if
	// types used in the contract need to be wrapped (necessitating a new contract that uses the wrapped types)
)

type TypeIdentifier struct {
	TypeKey      string
	Mode         TypeMode
	PointerDepth int // Only used for structs- how many *'s on this type?
	Type         gotypes.Type
}

func (t *TypeIdentifier) String() string {
	out := new(strings.Builder)

	switch t.Mode {
	case TypeStruct:
		fmt.Fprint(out, "STRUCT ")
	case TypeInterface:
		fmt.Fprint(out, "INTERFACE ")
	case TypeOther:
		fmt.Fprint(out, "OTHER ")
	}

	if t.PointerDepth > 0 {
		fmt.Fprintf(out, "PTRx%d ", t.PointerDepth)
	}

	fmt.Fprint(out, t.Type)

	return out.String()
}

func (t TypeIdentifier) TypeFileName() string {
	rootType := rootType(t.Type)
	namedRoot, isNamed := rootType.(*gotypes.Named)
	if isNamed {
		return strings.ToLower(fmt.Sprintf("%s_%s.go", namedRoot.Obj().Pkg().Name(), namedRoot.Obj().Name()))
	}

	return fmt.Sprintf("anon%s.go", hashFromType(rootType))
}

func (t TypeIdentifier) WrapperTypeName() string {
	rootType := rootType(t.Type)
	namedRoot, isNamed := rootType.(*gotypes.Named)
	if isNamed {
		return fmt.Sprintf("%s%s", strings.Title(namedRoot.Obj().Pkg().Name()), namedRoot.Obj().Name())
	}

	return fmt.Sprintf("Anon%s", hashFromType(rootType))
}

func (t TypeIdentifier) WrapFuncName() string {
	rootName := fmt.Sprintf("Wrap%s", t.WrapperTypeName())
	if t.Mode == TypeStruct {
		if t.PointerDepth == 0 {
			rootName = rootName + "Value"
		} else if t.PointerDepth > 1 {
			for i := 0; i < t.PointerDepth-1; i++ {
				rootName = rootName + "Ptr"
			}
		}
	}

	return rootName
}

type RootTypeInfo struct {
	RootType       TypeIdentifier    // This is the base, unpointered struct or whatever
	ClaimedMethods map[string]string // This is the list of method names that have already been used (and shouldn't be re-wrapped)
	// The value is the type key for the owning type- this is necessary because
	// we may assign a method to one type initially but allow another method to steal
	// it later
	HasDirectReceiver bool // If true, there is a method with a direct non-pointer receiver
}

func (r *RootTypeInfo) CanUseMethod(t gotypes.Type, method string) bool {
	ownerType, ok := r.ClaimedMethods[method]
	if !ok {
		return false
	}

	return ownerType == gotypes.TypeString(t, nil)
}

type TypeInfo struct {
	TypeId       TypeIdentifier // This is information for THIS type
	RootType     *RootTypeInfo
	Status       WrapStatus
	MethodToWrap []*gotypes.Selection
}

func (t *TypeInfo) String() string {
	return t.TypeId.String()
}
