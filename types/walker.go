package types

import (
	gotypes "go/types"
)

type TypeWalker struct {
	typeDB     *TypeDB
	queue      []*TypeInfo
	visited    map[string]bool
	packageSet map[string]bool
}

func NewTypeWalker(packages []string) *TypeWalker {
	packageSet := make(map[string]bool)
	for _, pkg := range packages {
		packageSet[pkg] = true
	}

	return &TypeWalker{
		typeDB:     newTypeDB(),
		queue:      []*TypeInfo{},
		packageSet: packageSet,
		visited:    make(map[string]bool),
	}
}

func (s *TypeWalker) QueueType(queueType gotypes.Type, dependentType gotypes.Type) {
	if !isNativeToPackageSet(queueType, s.packageSet) {
		return
	}

	queueTypeInfo := s.typeDB.AddType(queueType, dependentType)

	if s.visited[queueTypeInfo.TypeId.TypeKey] {
		return
	}

	s.visited[queueTypeInfo.TypeId.TypeKey] = true
	s.queue = append(s.queue, queueTypeInfo)

	// Queue any underlying types.  This will make sure that if we return []*Type somewhere, we'll queue []*Type, *Type, and Type
	for _, elementType := range elementTypes(queueType) {
		s.QueueType(elementType, dependentType)
	}
}

func (s *TypeWalker) dequeue() *TypeInfo {
	if len(s.queue) == 0 {
		return nil
	}

	item := s.queue[0]
	s.queue = s.queue[1:]
	return item
}
