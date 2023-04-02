package main

import (
	"bytes"
	"errors"
	"fmt"
	"go/format"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"text/template"

	gensc "github.com/FamilyChain/family/consensus/polybft/contractsapi"
	"github.com/FamilyChain/family/consensus/polybft/contractsapi/artifact"
	"github.com/umbracle/ethgo/abi"
)

const (
	abiTypeNameFormat  = "var %sABIType = abi.MustNewType(\"%s\")"
	eventNameFormat    = "%sEvent"
	functionNameFormat = "%sFn"
)

type generatedData struct {
	resultString []string
	structs      []string
}

func main() {
	cases := []struct {
		contractName string
		artifact     *artifact.Artifact
		functions    []string
		events       []string
	}{
		{
			"StateReceiver",
			gensc.StateReceiver,
			[]string{
				"commit",
				"execute",
			},
			[]string{
				"StateSyncResult",
				"NewCommitment",
			},
		},
		{
			"ChildValidatorSet",
			gensc.ChildValidatorSet,
			[]string{
				"commitEpoch",
				"initialize",
				"addToWhitelist",
				"register",
			},
			[]string{
				"NewValidator",
				"Staked",
				"Delegated",
				"Unstaked",
				"Undelegated",
				"AddedToWhitelist",
				"Withdrawal",
			},
		},
		{
			"StateSender",
			gensc.StateSender,
			[]string{
				"syncState",
			},
			[]string{
				"StateSynced",
			},
		},
		{
			"L2StateSender",
			gensc.L2StateSender,
			[]string{},
			[]string{
				"L2StateSynced",
			},
		},
		{
			"CheckpointManager",
			gensc.CheckpointManager,
			[]string{
				"submit",
				"initialize",
				"getCheckpointBlock",
			},
			[]string{},
		},
		{
			"ExitHelper",
			gensc.ExitHelper,
			[]string{
				"exit",
			},
			[]string{},
		},
		{
			"ChildERC20Predicate",
			gensc.ChildERC20Predicate,
			[]string{
				"initialize",
				"withdrawTo",
			},
			[]string{},
		},
		{
			"NativeERC20",
			gensc.NativeERC20,
			[]string{
				"initialize",
			},
			[]string{},
		},
		{
			"NativeERC20Mintable",
			gensc.NativeERC20Mintable,
			[]string{
				"initialize",
			},
			[]string{},
		},
		{
			"RootERC20Predicate",
			gensc.RootERC20Predicate,
			[]string{
				"initialize",
				"depositTo",
			},
			[]string{},
		},
		{
			"RootERC20",
			gensc.RootERC20,
			[]string{
				"approve",
				"mint",
			},
			[]string{},
		},
	}

	generatedData := &generatedData{}

	for _, c := range cases {
		for _, method := range c.functions {
			if err := generateFunction(generatedData, c.contractName, c.artifact.Abi.Methods[method]); err != nil {
				log.Fatal(err)
			}
		}

		for _, event := range c.events {
			if err := generateEvent(generatedData, c.contractName, c.artifact.Abi.Events[event]); err != nil {
				log.Fatal(err)
			}
		}
	}

	str := `// Code generated by scapi/gen. DO NOT EDIT.
package contractsapi

import (
	"math/big"

	"github.com/FamilyChain/family/types"
	"github.com/umbracle/ethgo/abi"
	"github.com/umbracle/ethgo"
)

`
	str += strings.Join(generatedData.resultString, "\n")

	output, err := format.Source([]byte(str))
	if err != nil {
		fmt.Println(str)
		log.Fatal(err)
	}

	if err := ioutil.WriteFile("./consensus/polybft/contractsapi/contractsapi.go", output, 0600); err != nil {
		log.Fatal(err)
	}
}

func getInternalType(paramName string, paramAbiType *abi.Type) string {
	internalType := paramAbiType.InternalType()
	if internalType == "" {
		internalType = strings.Title(paramName)
	} else {
		internalType = strings.TrimSuffix(internalType, "[]")      // remove [] if it's struct array
		internalType = strings.TrimPrefix(internalType, "struct ") // remove struct prefix
		// if struct is taken from an interface (ICheckpoint.Validator), remove interface
		parts := strings.Split(internalType, ".")
		if len(parts) > 1 {
			internalType = parts[1]
		}
	}

	return internalType
}

// generateType generates code for structs used in smart contract functions and events
func generateType(generatedData *generatedData, name string, obj *abi.Type, res *[]string) (string, error) {
	if obj.Kind() != abi.KindTuple {
		return "", errors.New("type not expected")
	}

	internalType := getInternalType(name, obj)
	generatedData.structs = append(generatedData.structs, internalType)

	str := []string{
		"type " + internalType + " struct {",
	}

	for _, tupleElem := range obj.TupleElems() {
		elem := tupleElem.Elem

		var typ string

		if elem.Kind() == abi.KindTuple {
			// Struct
			nestedType, err := generateNestedType(generatedData, tupleElem.Name, elem, res)
			if err != nil {
				return "", err
			}

			typ = nestedType
		} else if elem.Kind() == abi.KindSlice && elem.Elem().Kind() == abi.KindTuple {
			// []Struct
			nestedType, err := generateNestedType(generatedData, getInternalType(tupleElem.Name, elem), elem.Elem(), res)
			if err != nil {
				return "", err
			}

			typ = "[]" + nestedType
		} else if elem.Kind() == abi.KindArray && elem.Elem().Kind() == abi.KindTuple {
			// [n]Struct
			nestedType, err := generateNestedType(generatedData, getInternalType(tupleElem.Name, elem), elem.Elem(), res)
			if err != nil {
				return "", err
			}

			typ = "[" + strconv.Itoa(elem.Size()) + "]" + nestedType
		} else if elem.Kind() == abi.KindAddress {
			// for address use the native `types.Address` type instead of `ethgo.Address`. Note that
			// this only works for simple types and not for []address inputs. This is good enough since
			// there are no kinds like that in our smart contracts.
			typ = "types.Address"
		} else {
			// for the rest of the types use the go type returned by abi
			typ = elem.GoType().String()
		}

		// []byte and [n]byte get rendered as []uint68 and [n]uint8, since we do not have any
		// uint8 internally in polybft, we can use regexp to replace those values with the
		// correct byte representation
		typ = strings.Replace(typ, "[32]uint8", "types.Hash", -1)
		typ = strings.Replace(typ, "]uint8", "]byte", -1)

		// Trim the leading _ from name if it exists
		fieldName := strings.TrimPrefix(tupleElem.Name, "_")

		// Replacement of Id for ID to make the linter happy
		fieldName = strings.Title(fieldName)
		fieldName = strings.Replace(fieldName, "Id", "ID", -1)

		str = append(str, fmt.Sprintf("%s %s `abi:\"%s\"`", fieldName, typ, tupleElem.Name))
	}

	str = append(str, "}")
	*res = append(*res, strings.Join(str, "\n"))

	return internalType, nil
}

// generateNestedType generates code for nested types found in smart contracts structs
func generateNestedType(generatedData *generatedData, name string, obj *abi.Type, res *[]string) (string, error) {
	for _, s := range generatedData.structs {
		if s == name {
			// do not generate the same type again if it's already generated
			// this happens when two functions use the same struct type as one of its parameters
			return "*" + name, nil
		}
	}

	result, err := generateType(generatedData, name, obj, res)
	if err != nil {
		return "", err
	}

	*res = append(*res, fmt.Sprintf(abiTypeNameFormat, result, obj.Format(true)))

	nestedTypeFunctions, err := generateAbiFuncsForNestedType(result)
	if err != nil {
		return "", err
	}

	*res = append(*res, nestedTypeFunctions)

	return "*" + result, nil
}

// generateAbiFuncsForNestedType generates necessary functions for nested types smart contracts interaction
func generateAbiFuncsForNestedType(name string) (string, error) {
	tmpl := `func ({{.Sig}} *{{.TName}}) EncodeAbi() ([]byte, error) {
		return {{.Name}}ABIType.Encode({{.Sig}})
	}
	
	func ({{.Sig}} *{{.TName}}) DecodeAbi(buf []byte) error {
		return decodeStruct({{.Name}}ABIType, buf, &{{.Sig}})
	}`

	title := strings.Title(name)

	inputs := map[string]interface{}{
		"Sig":   strings.ToLower(string(name[0])),
		"Name":  title,
		"TName": title,
	}

	return renderTmpl(tmpl, inputs)
}

// generateEvent generates code for smart contract events
func generateEvent(generatedData *generatedData, contractName string, event *abi.Event) error {
	name := fmt.Sprintf(eventNameFormat, event.Name)
	res := []string{}

	_, err := generateType(generatedData, name, event.Inputs, &res)
	if err != nil {
		return err
	}

	// write encode/decode functions
	tmplStr := `
{{range .Structs}}
	{{.}}
{{ end }}

func (*{{.TName}}) Sig() ethgo.Hash {
	return {{.ContractName}}.Abi.Events["{{.Name}}"].ID()
}

func (*{{.TName}}) Encode(inputs interface{}) ([]byte, error) {
	return {{.ContractName}}.Abi.Events["{{.Name}}"].Inputs.Encode(inputs)
}

func ({{.Sig}} *{{.TName}}) ParseLog(log *ethgo.Log) (bool, error) {
	if (!{{.ContractName}}.Abi.Events["{{.Name}}"].Match(log)) {
		return false, nil
	}

	return true, decodeEvent({{.ContractName}}.Abi.Events["{{.Name}}"], log, {{.Sig}})
}`

	inputs := map[string]interface{}{
		"Structs":      res,
		"Sig":          strings.ToLower(string(name[0])),
		"Name":         event.Name,
		"TName":        strings.Title(name),
		"ContractName": contractName,
	}

	renderedString, err := renderTmpl(tmplStr, inputs)
	if err != nil {
		return err
	}

	generatedData.resultString = append(generatedData.resultString, renderedString)

	return nil
}

// generateFunction generates code for smart contract function and its parameters
func generateFunction(generatedData *generatedData, contractName string, method *abi.Method) error {
	methodName := fmt.Sprintf(functionNameFormat, strings.Title(method.Name+contractName))
	res := []string{}

	_, err := generateType(generatedData, methodName, method.Inputs, &res)
	if err != nil {
		return err
	}

	// write encode/decode functions
	tmplStr := `
{{range .Structs}}
	{{.}}
{{ end }}

func ({{.Sig}} *{{.TName}}) Sig() []byte {
	return {{.ContractName}}.Abi.Methods["{{.Name}}"].ID()
}

func ({{.Sig}} *{{.TName}}) EncodeAbi() ([]byte, error) {
	return {{.ContractName}}.Abi.Methods["{{.Name}}"].Encode({{.Sig}})
}

func ({{.Sig}} *{{.TName}}) DecodeAbi(buf []byte) error {
	return decodeMethod({{.ContractName}}.Abi.Methods["{{.Name}}"], buf, {{.Sig}})
}`

	inputs := map[string]interface{}{
		"Structs":      res,
		"Sig":          strings.ToLower(string(methodName[0])),
		"Name":         method.Name,
		"ContractName": contractName,
		"TName":        strings.Title(methodName),
	}

	renderedString, err := renderTmpl(tmplStr, inputs)
	if err != nil {
		return err
	}

	generatedData.resultString = append(generatedData.resultString, renderedString)

	return nil
}

func renderTmpl(tmplStr string, inputs map[string]interface{}) (string, error) {
	tmpl, err := template.New("name").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("failed to load template: %w", err)
	}

	var tpl bytes.Buffer
	if err = tmpl.Execute(&tpl, inputs); err != nil {
		return "", fmt.Errorf("failed to render template: %w", err)
	}

	return tpl.String(), nil
}
