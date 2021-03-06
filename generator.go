package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"go/ast"
	"log"
	"os"
	"path"
	"strings"

	"github.com/mbrevoort/swagger/parser"
)

const (
	AVAILABLE_FORMATS = "go|swagger|asciidoc|markdown|confluence"
)

var apiPackage = flag.String("apiPackage", "", "The package that implements the API controllers, relative to $GOPATH/src")
var mainApiFile = flag.String("mainApiFile", "", "The file that contains the general API annotations, relative to $GOPATH/src")
var basePath = flag.String("basePath", "", "Web service base path")
var outputSpec = flag.String("output", "", "Output (path) for the generated file(s)")

var generatedFileTemplate = `
package main
//This file is generated automatically. Do not edit it manually.

import (
	"net/http"
	"strings"
)

func swaggerApiHandler(prefix string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resource := strings.TrimPrefix(r.RequestURI, prefix)
		resource = strings.Trim(resource, "/")

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET")

		if resource == "" {
			w.Write([]byte(swaggerResourceListing))
			return
		}

		if json, ok := swaggerApiDescriptions[resource]; ok {
			w.Write([]byte(json))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}
}


var swaggerResourceListing = {{resourceListing}}
var swaggerApiDescriptions = {{apiDescriptions}}
`

// It must return true if funcDeclaration is controller. We will try to parse only comments before controllers
func IsController(funcDeclaration *ast.FuncDecl) bool {
	return true
	// if len(*controllerClass) == 0 {
	// 	// Search every method
	// 	return true
	// }
	// if funcDeclaration.Recv != nil && len(funcDeclaration.Recv.List) > 0 {
	// 	if starExpression, ok := funcDeclaration.Recv.List[0].Type.(*ast.StarExpr); ok {
	// 		receiverName := fmt.Sprint(starExpression.X)
	// 		matched, err := regexp.MatchString(string(*controllerClass), receiverName)
	// 		if err != nil {
	// 			log.Fatalf("The -controllerClass argument is not a valid regular expression: %v\n", err)
	// 		}
	// 		return matched
	// 	}
	// }
	// return false
}

func generateSwaggerDocs(parser *parser.Parser) {
	fd, err := os.Create(path.Join("./", "swaggerSpec.go"))
	if err != nil {
		log.Fatalf("Can not create document file: %v\n", err)
	}
	defer fd.Close()

	var apiDescriptions bytes.Buffer
	for apiKey, apiDescription := range parser.TopLevelApis {
		apiDescriptions.WriteString("\"" + apiKey + "\":")

		apiDescriptions.WriteString("`")
		json, err := json.MarshalIndent(apiDescription, "", "    ")
		if err != nil {
			log.Fatalf("Can not serialise []ApiDescription to JSON: %v\n", err)
		}
		apiDescriptions.Write(json)
		apiDescriptions.WriteString("`,")
	}

	doc := strings.Replace(generatedFileTemplate, "{{resourceListing}}", "`"+string(parser.GetResourceListingJson())+"`", -1)
	doc = strings.Replace(doc, "{{apiDescriptions}}", "map[string]string{"+apiDescriptions.String()+"}", -1)

	fd.WriteString(doc)
}

func InitParser() *parser.Parser {
	parser := parser.NewParser()

	parser.BasePath = *basePath
	parser.IsController = IsController

	parser.TypesImplementingMarshalInterface["NullString"] = "string"
	parser.TypesImplementingMarshalInterface["NullInt64"] = "int"
	parser.TypesImplementingMarshalInterface["NullFloat64"] = "float"
	parser.TypesImplementingMarshalInterface["NullBool"] = "bool"

	return parser
}

func main() {
	flag.Parse()

	if *mainApiFile == "" {
		*mainApiFile = *apiPackage + "/main.go"
	}
	if *apiPackage == "" {
		flag.PrintDefaults()
		return
	}

	parser := InitParser()
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		log.Fatalf("Please, set $GOPATH environment variable\n")
	}

	log.Println("Start parsing")
	parser.ParseGeneralApiInfo(path.Join(gopath, "src", *mainApiFile))
	parser.ParseApi(*apiPackage)
	log.Println("Finish parsing")

	generateSwaggerDocs(parser)

}
