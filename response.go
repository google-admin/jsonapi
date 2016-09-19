package jsonapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"time"
)

var (
	// ErrBadJSONAPIStructTag is returned when the Struct field's JSON API
	// annotation is invalid.
	ErrBadJSONAPIStructTag = errors.New("Bad jsonapi struct tag format")
	// ErrBadJSONAPIID is returned when the Struct JSON API annotated "id" field
	// was not a valid numeric type.
	ErrBadJSONAPIID = errors.New("id should be either string, int or uint")
	// ErrExpectedSlice is returned when a variable or arugment was expected to
	// be a slice of *Structs; MarshalMany will return this error when its
	// interface{} argument is invalid.
	ErrExpectedSlice = errors.New("models should be a slice of struct pointers")
)

// MarshalOnePayload writes a jsonapi response with one, with related records
// sideloaded, into "included" array. This method encodes a response for a
// single record only. Hence, data will be a single record rather than an array
// of records.  If you want to serialize many records, see, MarshalManyPayload.
//
// See UnmarshalPayload for usage example.
//
// model interface{} should be a pointer to a struct.
func MarshalOnePayload(w io.Writer, model interface{}) error {
	payload, err := MarshalOne(model)
	if err != nil {
		return err
	}

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		return err
	}

	return nil
}

// MarshalOnePayloadWithoutIncluded writes a jsonapi response with one object,
// without the related records sideloaded into "included" array. If you want to
// serialzie the realtions into the "included" array see MarshalOnePayload.
//
// model interface{} should be a pointer to a struct.
func MarshalOnePayloadWithoutIncluded(w io.Writer, model interface{}) error {
	included := make(map[string]*Node)

	rootNode, err := visitModelNode(model, &included, true)
	if err != nil {
		return err
	}

	if err := json.NewEncoder(w).Encode(&OnePayload{Data: rootNode}); err != nil {
		return err
	}

	return nil
}

// MarshalOne does the same as MarshalOnePayload except it just returns the
// payload and doesn't write out results. Useful is you use your JSON rendering
// library.
func MarshalOne(model interface{}) (*OnePayload, error) {
	included := make(map[string]*Node)

	rootNode, err := visitModelNode(model, &included, true)
	if err != nil {
		return nil, err
	}
	payload := &OnePayload{Data: rootNode}

	payload.Included = nodeMapValues(&included)

	return payload, nil
}

// MarshalManyPayload writes a jsonapi response with many records, with related
// records sideloaded, into "included" array. This method encodes a response for
// a slice of records, hence data will be an array of records rather than a
// single record.  To serialize a single record, see MarshalOnePayload
//
// For example you could pass it, w, your http.ResponseWriter, and, models, a
// slice of Blog struct instance pointers as interface{}'s to write to the
// response,
//
//	 func ListBlogs(w http.ResponseWriter, r *http.Request) {
//		 // ... fetch your blogs and filter, offset, limit, etc ...
//
//		 blogs := testBlogsForList()
//
//		 w.WriteHeader(200)
//		 w.Header().Set("Content-Type", "application/vnd.api+json")
//		 if err := jsonapi.MarshalManyPayload(w, blogs); err != nil {
//			 http.Error(w, err.Error(), 500)
//		 }
//	 }
//
//
// Visit https://github.com/google/jsonapi#list for more info.
//
// models interface{} should be a slice of struct pointers.
func MarshalManyPayload(w io.Writer, models interface{}) error {
	m, err := convertToSliceInterface(&models)
	if err != nil {
		return err
	}
	payload, err := MarshalMany(m)
	if err != nil {
		return err
	}

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		return err
	}

	return nil
}

// MarshalManyPayload writes a jsonapi response with many records, with related
// records sideloaded, into "included" array. This method encodes a response for
// a slice of records, hence data will be an array of records rather than a
// single record.  To serialize a single record, see MarshalOnePayload
//
// For example you could pass it, w, your http.ResponseWriter, and, models, a
// slice of Blog struct instance pointers as interface{}'s to write to the
// response,
//
//	 func ListBlogs(w http.ResponseWriter, r *http.Request) {
//		 // ... fetch your blogs and filter, offset, limit, etc ...
//
//		 blogs := testBlogsForList()
//
//		 w.WriteHeader(200)
//		 w.Header().Set("Content-Type", "application/vnd.api+json")
//		 if err := jsonapi.MarshalManyPayload(w, blogs); err != nil {
//			 http.Error(w, err.Error(), 500)
//		 }
//	 }
//
//
// Visit https://github.com/google/jsonapi#list for more info.
//
// models interface{} should be a slice of struct pointers.
func MarshalManyPayloadWithMeta(w io.Writer, models interface{}, meta interface{}) error {
	m, err := convertToSliceInterface(&models)
	if err != nil {
		return err
	}
	payload, err := marshalMany(m, meta)
	if err != nil {
		return err
	}

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		return err
	}

	return nil
}

// MarshalMany does the same as MarshalManyPayload except it just returns the
// payload and doesn't write out results. Useful is you use your JSON rendering
// library.
func MarshalMany(models []interface{}) (*ManyPayload, error) {
	return marshalMany(models, nil)
}

func marshalMany(models []interface{}, meta interface{}) (*ManyPayload, error) {
	var data []*Node
	included := make(map[string]*Node)

	for i := 0; i < len(models); i++ {
		model := models[i]

		node, err := visitModelNode(model, &included, true)
		if err != nil {
			return nil, err
		}
		data = append(data, node)
	}

	if len(models) == 0 {
		data = make([]*Node, 0)
	}

	payload := &ManyPayload{
		Data:     data,
		Included: nodeMapValues(&included),
	}

	if meta != nil {
		node, err := visitMetaNode(meta)
		if err != nil {
			return nil, err
		}
		payload.Meta = node
	}

	return payload, nil
}

// MarshalOnePayloadEmbedded - This method not meant to for use in
// implementation code, although feel free.  The purpose of this method is for
// use in tests.  In most cases, your request payloads for create will be
// embedded rather than sideloaded for related records. This method will
// serialize a single struct pointer into an embedded json response.  In other
// words, there will be no, "included", array in the json all relationships will
// be serailized inline in the data.
//
// However, in tests, you may want to construct payloads to post to create
// methods that are embedded to most closely resemble the payloads that will be
// produced by the client.  This is what this method is intended for.
//
// model interface{} should be a pointer to a struct.
func MarshalOnePayloadEmbedded(w io.Writer, model interface{}) error {
	rootNode, err := visitModelNode(model, nil, false)
	if err != nil {
		return err
	}

	payload := &OnePayload{Data: rootNode}

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		return err
	}

	return nil
}

func visitMetaNode(meta interface{}) (*map[string]interface{}, error) {
	node := make(map[string]interface{})

	var er error

	metaValue := reflect.ValueOf(meta).Elem()

	for i := 0; i < metaValue.NumField(); i++ {
		structField := metaValue.Type().Field(i)
		tag := structField.Tag.Get("jsonapi")
		if tag == "" {
			continue
		}

		fieldValue := metaValue.Field(i)

		args := strings.Split(tag, ",")

		if len(args) < 1 || len(args) > 2 {
			er = ErrBadJSONAPIStructTag
			break
		}

		var omitEmpty bool

		if len(args) == 2 {
			omitEmpty = args[1] == "omitempty"
			if !omitEmpty {
				er = ErrBadJSONAPIStructTag
				break
			}
		}

		if fieldValue.Type() == reflect.TypeOf(time.Time{}) {
			t := fieldValue.Interface().(time.Time)

			if t.IsZero() {
				continue
			}

			node[args[0]] = t.Unix()

		} else if fieldValue.Type() == reflect.TypeOf(new(time.Time)) {
			// A time pointer may be nil
			if fieldValue.IsNil() {
				if omitEmpty {
					continue
				}

				node[args[0]] = nil
			} else {
				tm := fieldValue.Interface().(*time.Time)

				if tm.IsZero() && omitEmpty {
					continue
				}

				node[args[0]] = tm.Unix()
			}
		} else {
			// Dealing with a fieldValue that is not a time
			emptyValue := reflect.Zero(fieldValue.Type())

			// See if we need to omit this field
			if omitEmpty && fieldValue.Interface() == emptyValue.Interface() {
				continue
			}

			strAttr, ok := fieldValue.Interface().(string)
			if ok {
				node[args[0]] = strAttr
			} else {
				node[args[0]] = fieldValue.Interface()
			}
		}
	}

	if er != nil {
		return nil, er
	}

	return &node, nil
}

func visitModelNode(model interface{}, included *map[string]*Node, sideload bool) (*Node, error) {
	node := new(Node)

	var er error

	modelValue := reflect.ValueOf(model).Elem()

	for i := 0; i < modelValue.NumField(); i++ {
		structField := modelValue.Type().Field(i)
		tag := structField.Tag.Get("jsonapi")
		if tag == "" {
			continue
		}

		fieldValue := modelValue.Field(i)

		args := strings.Split(tag, ",")

		if len(args) < 1 {
			er = ErrBadJSONAPIStructTag
			break
		}

		annotation := args[0]

		if (annotation == clientIDAnnotation && len(args) != 1) ||
			(annotation != clientIDAnnotation && len(args) < 2) {
			er = ErrBadJSONAPIStructTag
			break
		}

		if annotation == "primary" {
			id := fieldValue.Interface()

			switch nID := id.(type) {
			case string:
				node.ID = nID
			case int:
				node.ID = strconv.Itoa(nID)
			case int64:
				node.ID = strconv.FormatInt(nID, 10)
			case uint64:
				node.ID = strconv.FormatUint(nID, 10)
			default:
				er = ErrBadJSONAPIID
				break
			}

			node.Type = args[1]
		} else if annotation == clientIDAnnotation {
			clientID := fieldValue.String()
			if clientID != "" {
				node.ClientID = clientID
			}
		} else if annotation == "attr" {
			var omitEmpty bool

			if len(args) > 2 {
				omitEmpty = args[2] == "omitempty"
			}

			if node.Attributes == nil {
				node.Attributes = make(map[string]interface{})
			}

			if fieldValue.Type() == reflect.TypeOf(time.Time{}) {
				t := fieldValue.Interface().(time.Time)

				if t.IsZero() {
					continue
				}

				node.Attributes[args[1]] = t.Unix()
			} else if fieldValue.Type() == reflect.TypeOf(new(time.Time)) {
				// A time pointer may be nil
				if fieldValue.IsNil() {
					if omitEmpty {
						continue
					}

					node.Attributes[args[1]] = nil
				} else {
					tm := fieldValue.Interface().(*time.Time)

					if tm.IsZero() && omitEmpty {
						continue
					}

					node.Attributes[args[1]] = tm.Unix()
				}
			} else {
				// Dealing with a fieldValue that is not a time
				emptyValue := reflect.Zero(fieldValue.Type())

				// See if we need to omit this field
				if omitEmpty && fieldValue.Interface() == emptyValue.Interface() {
					continue
				}

				strAttr, ok := fieldValue.Interface().(string)
				if ok {
					node.Attributes[args[1]] = strAttr
				} else {
					node.Attributes[args[1]] = fieldValue.Interface()
				}
			}
		} else if annotation == "relation" {
			isSlice := fieldValue.Type().Kind() == reflect.Slice

			if (isSlice && fieldValue.Len() < 1) || (!isSlice && fieldValue.IsNil()) {
				continue
			}

			if node.Relationships == nil {
				node.Relationships = make(map[string]interface{})
			}

			if isSlice {
				relationship, err := visitModelNodeRelationships(
					args[1],
					fieldValue,
					included,
					sideload,
				)

				if err == nil {
					d := relationship.Data
					if sideload {
						var shallowNodes []*Node

						for _, n := range d {
							appendIncluded(included, n)
							shallowNodes = append(shallowNodes, toShallowNode(n))
						}

						node.Relationships[args[1]] = &RelationshipManyNode{
							Data: shallowNodes,
						}
					} else {
						node.Relationships[args[1]] = relationship
					}
				} else {
					er = err
					break
				}
			} else {
				relationship, err := visitModelNode(
					fieldValue.Interface(),
					included,
					sideload)
				if err == nil {
					if sideload {
						appendIncluded(included, relationship)
						node.Relationships[args[1]] = &RelationshipOneNode{
							Data: toShallowNode(relationship),
						}
					} else {
						node.Relationships[args[1]] = &RelationshipOneNode{
							Data: relationship,
						}
					}
				} else {
					er = err
					break
				}
			}

		} else {
			er = ErrBadJSONAPIStructTag
			break
		}
	}

	if er != nil {
		return nil, er
	}

	return node, nil
}

func toShallowNode(node *Node) *Node {
	return &Node{
		ID:   node.ID,
		Type: node.Type,
	}
}

func visitModelNodeRelationships(relationName string, models reflect.Value, included *map[string]*Node, sideload bool) (*RelationshipManyNode, error) {
	var nodes []*Node

	if models.Len() == 0 {
		nodes = make([]*Node, 0)
	}

	for i := 0; i < models.Len(); i++ {
		n := models.Index(i).Interface()
		node, err := visitModelNode(n, included, sideload)
		if err != nil {
			return nil, err
		}

		nodes = append(nodes, node)
	}

	return &RelationshipManyNode{Data: nodes}, nil
}

func appendIncluded(m *map[string]*Node, nodes ...*Node) {
	included := *m

	for _, n := range nodes {
		k := fmt.Sprintf("%s,%s", n.Type, n.ID)

		if _, hasNode := included[k]; hasNode {
			continue
		}

		included[k] = n
	}
}

func nodeMapValues(m *map[string]*Node) []*Node {
	mp := *m
	nodes := make([]*Node, len(mp))

	i := 0
	for _, n := range mp {
		nodes[i] = n
		i++
	}

	return nodes
}

func convertToSliceInterface(i *interface{}) ([]interface{}, error) {
	vals := reflect.ValueOf(*i)
	if vals.Kind() != reflect.Slice {
		return nil, ErrExpectedSlice
	}
	var response []interface{}
	for x := 0; x < vals.Len(); x++ {
		response = append(response, vals.Index(x).Interface())
	}
	return response, nil
}
