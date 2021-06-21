package types

import (
	gotypes "go/types"
)

func (state *TypeWalker) walkSingleType(walkType *TypeInfo) {
	if walkType == nil {
		//Don't think this is possible, but better safe than sorry when doing an interface type query
		return
	}

	methodSetToScan := gotypes.NewMethodSet(walkType.TypeId.Type)

	// Find exported methods
	mustHardWrapIfWrapped := false
	for i := 0; i < methodSetToScan.Len(); i++ {
		method := methodSetToScan.At(i)
		methodName := method.Obj().Name()

		// Imagine the following:
		// type Root struct {}
		// func (r Root) ValFunc() {}
		// func (r *Root) PtrFunc() {}
		//
		// ValFunc will appear in the method sets for both Root and *Root.  In the *Root methodset,
		// there will be no way of telling that the "real" receiver is Root.  However, it will show as
		// indirect=false in the Root methodset, so we can use that as a cue to "steal" the method.
		if method.Obj().Exported() {
			_, alreadyClaimed := walkType.RootType.ClaimedMethods[methodName]
			if !alreadyClaimed || !method.Indirect() {
				walkType.MethodToWrap = append(walkType.MethodToWrap, method)
				walkType.RootType.ClaimedMethods[methodName] = walkType.TypeId.TypeKey
			}
		} else {
			// Interfaces with unexported methods must hard wrap (everything else has to hard wrap anyway)
			mustHardWrapIfWrapped = true
		}
	}

	if len(walkType.MethodToWrap) == 0 {
		//No exported methods- nothing to do
		return
	}

	// Loop through each method
	for _, wrappedMethod := range walkType.MethodToWrap {
		sig := wrappedMethod.Type().(*gotypes.Signature)

		//Loop through each return type
		for i := 0; i < sig.Results().Len(); i++ {
			returnType := sig.Results().At(i).Type()
			if IsError(returnType) {
				//Wrap this type if one of its method returns an error
				if walkType.TypeId.Mode == TypeInterface && !mustHardWrapIfWrapped {
					walkType.Status = WrapStatusSoft
				} else {
					walkType.Status = WrapStatusHard
				}
			} else {
				state.QueueType(returnType, walkType.TypeId.Type)
			}
		}
	}
}

func (state *TypeWalker) WalkTypes() *TypeDB {
	for nextItem := state.dequeue(); nextItem != nil; nextItem = state.dequeue() {
		state.walkSingleType(nextItem)
	}

	// Walk back up the dependency tree from all definitely-wrap types and see if we find any maybe-wrap types,
	// who should be wrapped
	state.typeDB.ResolveDependencies()

	return state.typeDB
}
