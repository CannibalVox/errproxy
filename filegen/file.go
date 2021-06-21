package filegen

import (
	"fmt"
	gotypes "go/types"
	"path/filepath"
	"strings"

	"github.com/CannibalVox/errproxy/jenutils"
	"github.com/CannibalVox/errproxy/types"
	"github.com/dave/jennifer/jen"
)

func paramName(param *gotypes.Var, paramIndex int) string {
	if param.Name() == "" || param.Name() == "_" {
		return fmt.Sprintf("p%d", paramIndex)
	}

	return param.Name()
}

type FileCreate struct {
	jen      *jen.File
	typeDB   *types.TypeDB
	fileName string
}

func NewFile(pkgName string, t *types.TypeInfo, db *types.TypeDB) *FileCreate {
	fileCreate := &FileCreate{
		jen:      jen.NewFile(strings.ToLower(pkgName)),
		fileName: t.TypeId.TypeFileName(),
		typeDB:   db,
	}

	fileCreate.jen.PackageComment("ErrProxy Generated File, DO NOT EDIT")

	// type [ElementTypeName] struct {
	//   inner [ElementType]
	//	 errorTransformer ErrorTransformer
	// }
	innerField := jenutils.Type(jen.Id("inner"), t.TypeId.Type)
	errTransformerField := jen.Id("errorTransformer").Qual("github.com/CannibalVox/errproxy", "ErrorTransformer")

	fileCreate.jen.Type().Id(t.RootType.RootType.WrapperTypeName()).Struct(innerField, errTransformerField)

	fileCreate.jen.Line()

	return fileCreate
}

func (f *FileCreate) requiresWrap(t gotypes.Type, minWrapStatus types.WrapStatus) (bool, *types.TypeInfo) {
	typeInfo := f.typeDB.LocateTypeInfo(t)
	if typeInfo == nil {
		return false, nil
	}

	if typeInfo != nil && typeInfo.Status >= minWrapStatus {
		return true, typeInfo
	}

	return false, typeInfo
}

func (f *FileCreate) addWrappedType(stmt *jen.Statement, t gotypes.Type) jen.Code {
	doWrap, typeInfo := f.requiresWrap(t, types.WrapStatusHard)

	if doWrap {
		for i := 0; i < typeInfo.TypeId.PointerDepth; i++ {
			stmt = stmt.Op("*")
		}

		if typeInfo.TypeId.Mode == types.TypeInterface {
			stmt = stmt.Op("*")
		}

		return stmt.Id(typeInfo.TypeId.WrapperTypeName())
	} else {
		return jenutils.Type(stmt, t)
	}
}

func (f *FileCreate) AppendType(t *types.TypeInfo) {
	// func Wrap[ElementTypeName](inner [ElementType], errorTransformer ErrorTransformer) *[ElementTypeName] {
	// return &[ElementTypeName]{
	//    inner: inner,
	//    errorTransformer: errorTransformer,
	//  }
	//}

	// Signature
	innerField := jenutils.Type(jen.Id("inner"), t.TypeId.Type)
	errTransformerField := jen.Id("errorTransformer").Qual("github.com/CannibalVox/errproxy", "ErrorTransformer")
	funcDeclaration :=
		f.jen.Func().
			Id(t.TypeId.WrapFuncName()).
			Params(innerField, errTransformerField)

	f.addWrappedType(funcDeclaration, t.TypeId.Type)
	// End Signature

	funcDeclaration.BlockFunc(func(g *jen.Group) {
		// if inner == nil { return nil }
		if t.TypeId.PointerDepth > 0 {
			//If this is a pointer type, we'll need to dereference it soon so check if we can
			g.If(jen.Id("inner").Op("==").Nil()).Block(jen.Return(jen.Nil()))
			g.Line()
		}

		// If we're accepting a pointer, we need to dereference it before assigning the struct field
		innerAssign := jen.Null()
		if t.TypeId.PointerDepth > 0 {
			innerAssign = jen.Op("*")
		}

		// If we're returning a pointer, we need to reference the struct we're returning
		structAssign := jen.Null()
		if t.TypeId.PointerDepth > 0 || t.TypeId.Mode == types.TypeInterface {
			structAssign = jen.Op("&")
		}

		// return &StructType {
		// 	inner: *inner,
		//  errorTransformer: errorTransformer,
		// }
		g.Return(structAssign.Id(t.TypeId.WrapperTypeName()).Values(
			jen.DictFunc(func(d jen.Dict) {
				d[jen.Id("inner")] = innerAssign.Id("inner")
				d[jen.Id("errorTransformer")] = jen.Id("errorTransformer")
			}),
		))
	})

	f.jen.Line()

	// Wrap all exported methods
	for _, methodInfo := range t.MethodToWrap {
		f.wrapMethod(t, methodInfo)
	}

	f.jen.Line()
}

func (f *FileCreate) wrapMethod(t *types.TypeInfo, methodInfo *gotypes.Selection) {
	sig, ok := methodInfo.Type().(*gotypes.Signature)
	if !ok {
		return
	}

	if !t.RootType.CanUseMethod(t.TypeId.Type, methodInfo.Obj().Name()) {
		return
	}

	//func (s [ElementTypeName]) [Method Signature] {
	// r0, r1 := s.inner.[Method Name]([Params])
	// return r0, r1
	//}
	params := []jen.Code{}
	for i := 0; i < sig.Params().Len(); i++ {
		param := sig.Params().At(i)

		paramName := jen.Id(paramName(param, i))
		if sig.Variadic() && i == sig.Params().Len()-1 {
			paramSlice := param.Type().(*gotypes.Slice)
			params = append(params, f.addWrappedType(paramName.Op("..."), paramSlice.Elem()))
		} else {
			params = append(params, f.addWrappedType(paramName, param.Type()))
		}
	}

	retVal := []jen.Code{}
	for i := 0; i < sig.Results().Len(); i++ {
		result := sig.Results().At(i)
		retVal = append(retVal, f.addWrappedType(jen.Null(), result.Type()))
	}

	// Method signature
	receiverName := methodInfo.Type().(*gotypes.Signature).Recv().Name()
	if receiverName == "" {
		// Interfaces have blank receiver names- let's find something that won't have collisions!
		receiverName = fmt.Sprintf("iFace%s", t.TypeId.WrapperTypeName())
	}

	receiverType := jen.Id(t.TypeId.WrapperTypeName())
	if t.TypeId.PointerDepth > 0 {
		receiverType = jen.Op("*").Add(receiverType)
	}

	wrappedMethod := f.jen.Func().Params(jen.Id(receiverName).Add(receiverType)).
		Id(methodInfo.Obj().Name()).
		Params(params...)

	if len(retVal) > 0 {
		wrappedMethod.Params(retVal...)
	}

	wrappedMethod.BlockFunc(func(g *jen.Group) {
		//r0, r1 := s.inner.[FuncName](p0, p1)
		callLine := g.Null()

		if len(retVal) > 0 {
			for i := 0; i < sig.Results().Len(); i++ {
				if i != 0 {
					callLine.Op(",")
				}

				callLine.Id(fmt.Sprintf("r%d", i))
			}

			callLine.Op(":=")
		}

		callLine.Id(receiverName).Dot("inner").Dot(methodInfo.Obj().Name()).CallFunc(func(g *jen.Group) {
			for i := 0; i < sig.Params().Len(); i++ {
				param := sig.Params().At(i)

				paramVal := paramName(param, i)

				var paramCodes *jen.Statement
				doWrap, paramTypeInfo := f.requiresWrap(param.Type(), types.WrapStatusHard)
				if doWrap && paramTypeInfo.TypeId.PointerDepth > 0 {
					paramCodes = g.Op("&").Id(paramVal).Dot("inner")
				} else if doWrap {
					paramCodes = g.Id(paramVal).Dot("inner")
				} else {
					paramCodes = g.Id(paramVal)
				}

				if sig.Variadic() && i == sig.Params().Len()-1 {
					paramCodes.Op("...")
				}
			}
		})

		if len(retVal) > 0 {
			//return r0, r1
			g.ReturnFunc(func(g *jen.Group) {
				for i := 0; i < sig.Results().Len(); i++ {
					result := sig.Results().At(i)
					retVar := jen.Id(fmt.Sprintf("r%d", i))
					doWrap, typeInfo := f.requiresWrap(result.Type(), types.WrapStatusSoft)

					// If error, return s.errorTransformer(r0)
					// If wrappable type, return WrapSomeType(r0)
					// otherwise just return r0
					if types.IsError(result.Type()) {
						g.Id(receiverName).Dot("errorTransformer").Call(retVar)
					} else if doWrap {
						g.Id(typeInfo.TypeId.WrapFuncName()).Call(retVar, jen.Id(receiverName).Dot("errorTransformer"))
					} else {
						g.Add(retVar)
					}
				}
			})
		}
	})

	f.jen.Line()
}

func (f *FileCreate) WriteFile(folder string) error {
	return f.jen.Save(filepath.Join(folder, f.fileName))
}

func (f *FileCreate) String() string {
	return f.fileName
}
