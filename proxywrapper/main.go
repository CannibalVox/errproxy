package main

import (
	"flag"
	gotypes "go/types"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/CannibalVox/errproxy/filegen"
	"github.com/CannibalVox/errproxy/types"
)

var inputPackageName string
var additionalInputPackages string
var typeName string
var outputPath string
var outputPackage string

func init() {
	flag.StringVar(&inputPackageName, "input", "", "package URL to read the type from")
	flag.StringVar(&additionalInputPackages, "additionalPkgs", "", "comma separated list of package URLs- types in these packages should be wrapped if located in the dendency graph of the original type")
	flag.StringVar(&typeName, "type", "", "type to read & wrap")
	flag.StringVar(&outputPath, "output", "", "package URL to write generated types to")
	flag.StringVar(&outputPackage, "pkg", "", "package name to use for generated code- defaults to folder name")
}

func loadType(inputPackage string, inputType string, allPackages []string) (gotypes.Type, []string) {
	// Load requested package
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedDeps | packages.NeedImports | packages.NeedTypes | packages.NeedTypesInfo,
	}

	// We need to get the fully qualified package path for the input package (to make sure we're getting the type)
	// form the right place, and we also need the fully qualified package paths for all packages (just to use them for filtering)
	// User may or may not provide them to us, so we'll load packages & get them from there
	fullQualifiedPackages := []string{}
	singleInputPackage, err := packages.Load(cfg, inputPackage)
	if err != nil {
		log.Fatalln(err)
	}

	inputPackage = singleInputPackage[0].PkgPath

	pkgs, err := packages.Load(cfg, allPackages...)
	if err != nil {
		log.Fatalln(err)
	}

	// Fail on error, otherwise scoop up requested type
	packages.PrintErrors(pkgs)
	var locatedTypeDef gotypes.Object
	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			os.Exit(1)
		}

		fullQualifiedPackages = append(fullQualifiedPackages, pkg.PkgPath)

		if pkg.PkgPath == inputPackage {
			locatedTypeDef = pkg.Types.Scope().Lookup(typeName)
		}
	}

	if locatedTypeDef == nil {
		log.Fatalf("Type '%s' could not be located in package %s\n", typeName, inputPackage)
	}

	return locatedTypeDef.Type(), fullQualifiedPackages
}

func deleteGeneratedFiles(generationPath string) error {
	err := filepath.WalkDir(generationPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if path == generationPath {
			return nil
		}

		if d.IsDir() {
			return nil
		}

		if filepath.Ext(path) != ".go" {
			return nil
		}

		text, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}

		if !strings.HasPrefix(string(text), "// ErrProxy Generated File, DO NOT EDIT") {
			return nil
		}

		os.Remove(path)

		return nil
	})

	return err
}

func main() {
	flag.Parse()

	if inputPackageName == "" || typeName == "" || outputPath == "" {
		flag.Usage()
		return
	}

	outputPath, err := filepath.Abs(outputPath)
	if err != nil {
		log.Fatalln(err)
	}

	splitAddtlPkgs := strings.Split(additionalInputPackages, ",")
	allPackages := []string{inputPackageName}
	for _, pkg := range splitAddtlPkgs {
		if pkg != "" {
			allPackages = append(allPackages, pkg)
		}
	}

	// The user may not have entered a fully-qualified pkg, so load the pkgs they asked for and build a new pkg list
	// from the fully-qualified names

	typeToWrap, fullyQualifiedPackages := loadType(inputPackageName, typeName, allPackages)
	walker := types.NewTypeWalker(fullyQualifiedPackages)

	switch typeToWrap.(type) {
	case *gotypes.Named, *gotypes.Struct:
		typeToWrap = gotypes.NewPointer(typeToWrap)
	}
	walker.QueueType(typeToWrap, nil)
	typeDB := walker.WalkTypes()

	_, err = os.Stat(outputPath)
	if err != nil && !os.IsNotExist(err) {
		log.Fatalln(err)
	}

	if !os.IsNotExist(err) {
		if err != nil {
			log.Fatalln(err)
		}

		// The folder already exists, so we should check if all files within are ErrProxy-generated go files
		// If not, error out.  If so, delete the folder
		err := deleteGeneratedFiles(outputPath)
		if err != nil {
			log.Fatalln(err)
		}
	} else {
		err = os.MkdirAll(outputPath, 0755)
		if err != nil {
			log.Fatalln(err)
		}
	}

	// If no -pkg flag was passed in, just break off the last folder in the path as the package name
	if outputPackage == "" {
		_, outputPackage = filepath.Split(outputPath)
	}

	// Create files, base types, wrapper function for base types
	fileGens := make(map[string]*filegen.FileCreate)
	err = typeDB.WalkRootTypes(func(t *types.RootTypeInfo) error {
		fileGen := filegen.NewFile(outputPackage, typeDB.LocateTypeInfo(t.RootType.Type), typeDB)
		fileGens[t.RootType.TypeKey] = fileGen

		return nil
	})

	if err != nil {
		log.Fatalln(err)
	}

	// Wrap all methods for each type
	err = typeDB.WalkAllTypes(func(t *types.TypeInfo) error {
		fileGen := fileGens[t.RootType.RootType.TypeKey]
		fileGen.AppendType(t)

		return nil
	})

	if err != nil {
		log.Fatalln(err)
	}

	//Output generated files
	for _, fileGen := range fileGens {
		err := fileGen.WriteFile(outputPath)
		if err != nil {
			log.Fatalln(err)
		}
	}

	log.Printf("Successfully generated wrapper in %s", outputPath)
}
