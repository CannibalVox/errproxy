package types

import gotypes "go/types"

type TypeDB struct {
	typesByKey  map[string]*TypeInfo
	typesByRoot map[string]*RootTypeInfo
	dependents  map[string]map[string]bool
}

func newTypeDB() *TypeDB {
	return &TypeDB{
		typesByKey:  make(map[string]*TypeInfo),
		typesByRoot: make(map[string]*RootTypeInfo),
		dependents:  make(map[string]map[string]bool),
	}
}

func createTypeIdentifier(t gotypes.Type) TypeIdentifier {
	typeKey := gotypes.TypeString(t, nil)
	typeMode, typeDepth := queryType(t)
	return TypeIdentifier{
		TypeKey:      typeKey,
		Mode:         typeMode,
		PointerDepth: typeDepth,
		Type:         t,
	}
}

func (t *TypeDB) AddType(actualType gotypes.Type, dependentType gotypes.Type) *TypeInfo {
	dependentTypeKey := gotypes.TypeString(dependentType, nil)

	actualTypeID := createTypeIdentifier(actualType)

	typeInfo, ok := t.typesByKey[actualTypeID.TypeKey]
	if !ok {
		rootType := rootType(actualType)
		rootTypeID := createTypeIdentifier(rootType)

		rootTypeInfo, foundRootType := t.typesByRoot[rootTypeID.TypeKey]
		if !foundRootType {
			rootTypeInfo = &RootTypeInfo{
				RootType:       rootTypeID,
				ClaimedMethods: make(map[string]string),
			}
			t.typesByRoot[rootTypeID.TypeKey] = rootTypeInfo
		}

		typeInfo = &TypeInfo{
			TypeId:       actualTypeID,
			RootType:     rootTypeInfo,
			Status:       WrapStatusDont,
			MethodToWrap: []*gotypes.Selection{},
		}
		t.typesByKey[typeInfo.TypeId.TypeKey] = typeInfo
		t.dependents[typeInfo.TypeId.TypeKey] = make(map[string]bool)
	}

	t.dependents[actualTypeID.TypeKey][dependentTypeKey] = true

	return typeInfo
}

// ResolveDependencies flows type statuses up the dependency graph, so if a "WrapStatusDont" type
// depends on a "WrapStatusHard" type, it becomes a "WrapStatusHard" type
// The only exception is that non-interfaces that depend on a WrapStatusSoft type become a WrapStatusHard
// type, since only interfaces can be soft.  Soft indicates that the wrapped interface will be reused
// with a wrapping implementation.  Hard indicates a brand new type is necessary.
func (t *TypeDB) ResolveDependencies() {
	for typeKey := range t.typesByKey {
		t.walkDependency(typeKey)
	}
}

// Recurse method for ResolveDependencies, see that method for more info
func (t *TypeDB) walkDependency(typeKey string) {
	typeInfo, ok := t.typesByKey[typeKey]
	if !ok {
		return
	}

	dependents, ok := t.dependents[typeKey]
	if !ok {
		return
	}

	status := typeInfo.Status

	for dependentKey := range dependents {
		dependentType, ok := t.typesByKey[dependentKey]
		if !ok {
			continue
		}

		if dependentType.Status >= status {
			continue
		}

		if status == WrapStatusSoft && dependentType.TypeId.Mode != TypeInterface {
			dependentType.Status = WrapStatusHard
		} else {
			dependentType.Status = status
		}

		t.walkDependency(dependentKey)
	}
}

func (t *TypeDB) WalkRootTypes(visitor func(t *RootTypeInfo) error) error {
	rootVisited := make(map[string]bool)
	for _, typeInfo := range t.typesByKey {
		if typeInfo.Status > WrapStatusDont {
			rootKey := typeInfo.RootType.RootType.TypeKey
			_, visited := rootVisited[rootKey]
			if !visited {
				err := visitor(typeInfo.RootType)
				if err != nil {
					return err
				}
				rootVisited[rootKey] = true
			}
		}
	}

	return nil
}

func (t *TypeDB) WalkAllTypes(visitor func(t *TypeInfo) error) error {
	for _, typeInfo := range t.typesByKey {
		if typeInfo.Status > WrapStatusDont {
			err := visitor(typeInfo)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (t *TypeDB) LocateTypeInfo(locateType gotypes.Type) *TypeInfo {
	foundType, ok := t.typesByKey[gotypes.TypeString(locateType, nil)]
	if !ok {
		return nil
	}

	return foundType
}
