package types

import (
	"crypto/sha1"
	"fmt"
	gotypes "go/types"
)

func queryType(walkType gotypes.Type) (mode TypeMode, ptrDepth int) {
	switch c := walkType.(type) {
	case *gotypes.Named:
		return queryType(c.Underlying())
	case *gotypes.Interface:
		return TypeInterface, 0
	case *gotypes.Struct:
		return TypeStruct, 0
	case *gotypes.Pointer:
		mode, depth := queryType(c.Elem())
		return mode, depth + 1
	default:
		return TypeOther, 0
	}
}

func rootType(walkType gotypes.Type) gotypes.Type {
	elemType, ok := walkType.(*gotypes.Pointer)
	if ok {
		return rootType(elemType.Elem())
	}

	return walkType
}

func elementTypes(walkType gotypes.Type) []gotypes.Type {
	switch c := walkType.(type) {
	case *gotypes.Array:
		return []gotypes.Type{c.Elem()}
	case *gotypes.Slice:
		return []gotypes.Type{c.Elem()}
	case *gotypes.Pointer:
		return []gotypes.Type{c.Elem()}
	case *gotypes.Map:
		return []gotypes.Type{c.Key(), c.Elem()}
	case *gotypes.Named:
		underlying, ptrDepth := queryType(c.Underlying())
		if (underlying != TypeStruct && underlying != TypeInterface) || ptrDepth > 0 {
			return []gotypes.Type{c.Underlying()}
		}
		return []gotypes.Type{}
	case *gotypes.Chan:
		return []gotypes.Type{c.Elem()}
	}

	return nil
}

func isNativeToPackageSet(walkType gotypes.Type, packageSet map[string]bool) bool {
	switch c := walkType.(type) {
	case *gotypes.Named:
		_, isNative := packageSet[c.Obj().Pkg().Path()]
		return isNative
	default:
		elementTypes := elementTypes(walkType)
		for _, elementType := range elementTypes {
			if isNativeToPackageSet(elementType, packageSet) {
				return true
			}
		}
	}

	return false
}

func hashFromType(t gotypes.Type) string {
	input := gotypes.TypeString(t, nil)
	hash := sha1.New()
	hash.Write([]byte(input))
	anonHash := hash.Sum(nil)
	return fmt.Sprintf("%x", anonHash)
}

var errorType = gotypes.Universe.Lookup("error").Type()

func IsError(t gotypes.Type) bool {
	return gotypes.Identical(t, errorType)
}
