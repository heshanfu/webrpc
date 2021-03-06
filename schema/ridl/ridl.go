package ridl

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/webrpc/webrpc/schema"
)

func tokenize(input string) ([]token, error) {
	lx := newLexer(string(input))

	tokens := []token{}
	for {
		tok := <-lx.tokens
		if tok.tt == tokenSpace {
			continue
		}
		if tok.tt == tokenEOF {
			break
		}
		tokens = append(tokens, tok)
	}

	return tokens, nil
}

func Parse(input string) (*schema.WebRPCSchema, error) {
	p, err := newParser(input)
	if err != nil {
		return nil, err
	}
	if err = p.run(); err != nil {
		return nil, err
	}

	if p.tree.definitions["webrpc"] == nil {
		return nil, errors.New(`missing "webrpc" declaration`)
	}
	webrpcInputVersion := p.tree.definitions["webrpc"].value()
	if webrpcInputVersion != schema.VERSION {
		return nil, errors.New("invalid webrpc declaration in ridl file")
	}

	if p.tree.definitions["name"] == nil {
		return nil, errors.New(`missing "name" declaration`)
	}

	if p.tree.definitions["version"] == nil {
		return nil, errors.New(`missing "version" declaration`)
	}

	s := &schema.WebRPCSchema{
		Schema:  webrpcInputVersion,
		Name:    p.tree.definitions["name"].value(),
		Version: p.tree.definitions["version"].value(),
	}

	if len(p.tree.imports) > 0 {
		s.Imports = []string{}
		for _, tok := range p.tree.imports {
			s.Imports = append(s.Imports, tok.val)
		}
	}

	if len(p.tree.enums) > 0 {
		if s.Messages == nil {
			s.Messages = []*schema.Message{}
		}
		for _, enum := range p.tree.enums {
			fields := []*schema.MessageField{}

			var varType schema.VarType
			err := schema.ParseVarTypeExpr(s, enum.enumType.val, &varType)
			if err != nil {
				return nil, fmt.Errorf("unknown data type: %v", enum.enumType)
			}

			for i := range enum.values {
				value := enum.values[i]
				field := &schema.MessageField{
					Name: schema.VarName(value.left.val),
					Type: &varType,
				}
				if value.right != nil {
					field.Value = value.right.val
				} else {
					field.Value = strconv.Itoa(i)
				}
				fields = append(fields, field)
			}

			s.Messages = append(s.Messages, &schema.Message{
				Name:     schema.VarName(enum.name.val),
				Type:     schema.MessageType("enum"),
				Fields:   fields,
				EnumType: &varType,
			})
		}
	}

	if len(p.tree.messages) > 0 {
		if s.Messages == nil {
			s.Messages = []*schema.Message{}
		}
		for _, message := range p.tree.messages {
			fields := []*schema.MessageField{}

			for i := range message.fields {
				value := message.fields[i]

				var varType schema.VarType
				err := schema.ParseVarTypeExpr(s, value.right.val, &varType)
				if err != nil {
					return nil, fmt.Errorf("unknown data type: %v", value.right.val)
				}
				field := &schema.MessageField{
					Name:     schema.VarName(value.left.val),
					Optional: value.optional,
					Type:     &varType,
				}
				for _, meta := range value.meta {
					field.Meta = append(field.Meta, schema.MessageFieldMeta{
						meta.left.val: meta.right.val,
					})
				}
				fields = append(fields, field)
			}

			s.Messages = append(s.Messages, &schema.Message{
				Name:   schema.VarName(message.name.val),
				Type:   schema.MessageType("struct"),
				Fields: fields,
			})
		}
	}

	if len(p.tree.services) > 0 {
		if s.Services == nil {
			s.Services = []*schema.Service{}
		}
		for _, service := range p.tree.services {
			methods := []*schema.Method{}

			for i := range service.methods {
				value := service.methods[i]

				method := &schema.Method{
					Name:    schema.VarName(value.name.val),
					Inputs:  []*schema.MethodArgument{},
					Outputs: []*schema.MethodArgument{},
				}

				// add inputs
				for _, arg := range value.inputs {
					var varType schema.VarType
					err := schema.ParseVarTypeExpr(s, arg.right.val, &varType)
					if err != nil {
						return nil, fmt.Errorf("unknown data type: %v", arg.right.val)
					}
					methodArgument := &schema.MethodArgument{
						Type:     &varType,
						Stream:   arg.stream,
						Optional: arg.optional,
					}
					if arg.left != nil {
						methodArgument.Name = schema.VarName(arg.left.val)
					}
					method.Inputs = append(method.Inputs, methodArgument)
				}

				// add outputs
				for _, arg := range value.outputs {
					var varType schema.VarType
					err := schema.ParseVarTypeExpr(s, arg.right.val, &varType)
					if err != nil {
						return nil, fmt.Errorf("unknown data type: %v", arg.right.val)
					}
					methodArgument := &schema.MethodArgument{
						Type:     &varType,
						Stream:   arg.stream,
						Optional: arg.optional,
					}
					if arg.left != nil {
						methodArgument.Name = schema.VarName(arg.left.val)
					}
					method.Outputs = append(method.Outputs, methodArgument)
				}

				// push method
				methods = append(methods, method)
			}

			// push service
			s.Services = append(s.Services, &schema.Service{
				Name:    schema.VarName(service.name.val),
				Methods: methods,
			})
		}
	}

	// run through schema validator, last step to ensure all is good.
	err = s.Parse(nil)
	if err != nil {
		return s, err
	}

	return s, nil
}
