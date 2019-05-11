package helper

import (
	"github.com/davecgh/go-spew/spew"
	"github.com/hashicorp/hcl2/hcl"
	"github.com/hashicorp/hcl2/hcl/hclsyntax"
	"github.com/hashicorp/terraform/configs"
	"github.com/sourcegraph/go-lsp"
	"github.com/zclconf/go-cty/cty"
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"regexp"
	"strings"
	"unicode/utf8"
)

func CheckAndGetConfig(parser *configs.Parser, originalFile *os.File, line int, character int) (*configs.File, hcl.Diagnostics, int, *hclsyntax.Body) {
	fileText, _ := ioutil.ReadFile(originalFile.Name())
	result := make([]byte, 1)
	pos := FindOffset(string(fileText), line, character)

	tempFile, _ := ioutil.TempFile("/tmp", "check_tf_lsp")
	defer os.Remove(tempFile.Name())

	originalFile.ReadAt(result, int64(pos))

	if string(result) == "." {
		fileText[pos] = ' '

		fileText = []byte(strings.Replace(string(fileText), ". ", "  ", -1))
		fileText = []byte(strings.Replace(string(fileText), ".,", " ,", -1))
		tempFile.Truncate(0)
		tempFile.Seek(0, 0)
		tempFile.Write(fileText)

		resultConfig, diags := parser.LoadConfigFileOverride(tempFile.Name())
		testRes, _ := parser.LoadHCLFile(tempFile.Name())

		return resultConfig, diags, character - 1, testRes.(*hclsyntax.Body)
	}

	textLines := strings.Split(string(fileText), "\n")

	re := regexp.MustCompile("\\s+([A-Za-z]*)$")

	DumpLog(re.FindAll([]byte(textLines[line-1]), -1))
	if (line-1) < len(textLines) && re.FindAll([]byte(textLines[line-1]), -1) != nil && len(re.FindAll([]byte(textLines[line-1]), -1)) != 1 {
		textLines[line-1] = strings.Repeat(" ", utf8.RuneCountInString(textLines[line-1]))
		tempFile.Truncate(0)
		tempFile.Seek(0, 0)
		tempFile.Write([]byte(strings.Join(textLines, "\n")))
		DumpLog(textLines)
		resultConfig, diags := parser.LoadConfigFileOverride(tempFile.Name())
		testRes, _ := parser.LoadHCLFile(tempFile.Name())
		return resultConfig, diags, character, testRes.(*hclsyntax.Body)
	}

	testRes, _ := parser.LoadHCLFile(originalFile.Name())
	resultConfig, diags := parser.LoadConfigFileOverride(originalFile.Name())
	return resultConfig, diags, character, testRes.(*hclsyntax.Body)
}

func FindOffset(fileText string, line, column int) int {
	currentCol := 1
	currentLine := 1

	for offset, ch := range fileText {
		if currentLine == line && currentCol == column {
			return offset
		}

		if ch == '\n' {
			currentLine++
			currentCol = 1
		} else {
			currentCol++
		}

	}
	return -1
}

func DumpLog(res interface{}) {
	log.Println(spew.Sdump(res))
}

func GetType(t interface{}) reflect.Type {
	return reflect.TypeOf(t)
}

func ParseVariables(vars hcl.Traversal, configVars map[string]*configs.Variable, completionItems []lsp.CompletionItem) []lsp.CompletionItem {
	if len(vars) == 0 {
		for _, t := range configVars {
			completionItems = append(completionItems, lsp.CompletionItem{
				Label:  t.Name,
				Detail: " " + t.Type.FriendlyName(),
			})
		}
		return completionItems
	}

	currVar := configVars[vars[0].(hcl.TraverseAttr).Name]

	if currVar != nil {
		return parseVariables(vars[1:], &currVar.Type, completionItems)
	}
	return completionItems
}

func parseVariables(vars hcl.Traversal, configVarsType *cty.Type, completionItems []lsp.CompletionItem) []lsp.CompletionItem {
	if len(vars) == 0 {
		if configVarsType.IsObjectType() {
			for k, t := range configVarsType.AttributeTypes() {
				completionItems = append(completionItems, lsp.CompletionItem{
					Label:  k,
					Detail: " " + t.FriendlyName(),
				})
			}

			return completionItems
		}

		return completionItems
	}

	if !configVarsType.IsObjectType() {
		if et := configVarsType.MapElementType(); et != nil {
			return parseVariables(vars[1:], et, completionItems)
		}

		if et := configVarsType.ListElementType(); et != nil {
			return parseVariables(vars[1:], et, completionItems)
		}
	}

	varAttr := vars[0].(hcl.TraverseAttr)

	if configVarsType.HasAttribute(varAttr.Name) {
		attr := configVarsType.AttributeType(varAttr.Name)
		return parseVariables(vars[1:], &attr, completionItems)
	}

	return nil
}

func ParseOtherAttr(vars hcl.Traversal, configVarsType cty.Type, completionItems []lsp.CompletionItem) []lsp.CompletionItem {
	return parseVariables(vars, &configVarsType, completionItems)
}
