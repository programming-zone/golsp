
// Evaluator

package golsp

import (
	"math"
	"strconv"
	"fmt"
)

// comparePatternNode: Compare a node in a function pattern with an argument object
// `pattern`: the pattern node
// `arg`: the argument to compare with the pattern
// this function returns whether the argument matches the pattern node
func comparePatternNode(pattern STNode, arg GolspObject) bool {
	// identifiers i.e non-literal patterns match everything
	if pattern.Type == STNodeTypeIdentifier { return true }

	// literal patterns match arguments that have the same value
	if pattern.Type == STNodeTypeStringLiteral ||
		pattern.Type == STNodeTypeNumberLiteral {
		return arg.Value.Head == pattern.Head
	}

	// map patterns match if all the specified keys and values match
	// value-only matching i.e `[foo ( quux: "hello" )]` does not work yet
	if pattern.Type == STNodeTypeMap {
		if arg.Type != GolspObjectTypeMap { return false }

		for i, c := range pattern.Children {
			if c.Spread && c.Type == STNodeTypeIdentifier {
				return len(arg.MapKeys) >= i
			}
			if len(arg.MapKeys) <= i { return false }
			if c.Type == STNodeTypeStringLiteral || c.Type == STNodeTypeNumberLiteral {
				value, exists := arg.Map[c.Head]
				if !exists { return false }
				if c.Zip != nil {
					if !comparePatternNode(*c.Zip, value) { return false }
				}
			}
		}

		if len(arg.MapKeys) > len(pattern.Children) { return false }
	}

	// list patterns match if each of their elements match and the lists
	// are of the same length, after accounting for spreading
	if pattern.Type == STNodeTypeList {
		if arg.Type != GolspObjectTypeList { return false }

		for i, c := range pattern.Children {
			if c.Spread && c.Type == STNodeTypeIdentifier {
				return len(arg.Elements) >= i
			}
			if len(arg.Elements) <= i { return false }
			if !comparePatternNode(c, arg.Elements[i]) { return false }
		}

		if len(arg.Elements) > len(pattern.Children) { return false }
	}

	return true
}

// matchPatterns: Match a list of arguments to a particular function pattern
// `fn`: the function whose patterns to check
// `arguments`: the list of arguments to match to a pattern
// this function returns the index of the best-matching pattern in function's
// list of patterns
func matchPatterns(fn GolspFunction, arguments []GolspObject) int {
	patterns := fn.FunctionPatterns
	bestmatchscore := 0
	bestmatchindex := 0

	for i, p := range patterns {
		score := 0
		minlen := int(math.Min(float64(len(p)), float64(len(arguments))))

		for j := 0; j < minlen; j++ {
			if comparePatternNode(p[j], arguments[j]) { score++ }
		}

		if score > bestmatchscore {
			bestmatchscore = score
			bestmatchindex = i
		}
	}

	return bestmatchindex
}

// LookupIdentifier: lookup an identifier within a particular scope
// `scope`: the scope in which to search for the identifier
// `identifier`: the name of the identifier
// this function returns the object corresponding to the identifier
// or UNDEFINED
func LookupIdentifier(scope GolspScope, identifier string) GolspObject {
	obj, exists := scope.Identifiers[identifier]
	if exists { return obj }

	if scope.Parent != nil {
		return LookupIdentifier(*(scope.Parent), identifier)
	}

	return Builtins.Identifiers[UNDEFINED]
}

// MakeScope: construct a new child scope that descends from a parent scope
// `parent`: the parent scope
// this function returns a new GolspScope object whose Parent points to
// parent
func MakeScope(parent *GolspScope) GolspScope {
	newscope := GolspScope{
		Parent: parent,
		Identifiers: make(map[string]GolspObject),
	}

	return newscope
}

// copyFunction: Copy a GolspFunction object
// `fn`: the function to copy
// this function returns a copy of fn
func copyFunction(fn GolspFunction) GolspFunction {
	fncopy := GolspFunction{
		FunctionPatterns: make([][]STNode, len(fn.FunctionPatterns)),
		FunctionBodies: make([]STNode, len(fn.FunctionBodies)),
	}
	copy(fncopy.FunctionPatterns, fn.FunctionPatterns)
	copy(fncopy.FunctionBodies, fn.FunctionBodies)
	fncopy.BuiltinFunc = fn.BuiltinFunc

	return fncopy
}

// copyObject: Copy a GolspObject object
// `object`: the object to copy
// this function returns a copy of object. Note that it does not copy
// object.Value since that property is never modified
func copyObject(object GolspObject) GolspObject {
	newobject := GolspObject{
		Type: object.Type,
		Value: object.Value,
		Function: copyFunction(object.Function),
		Elements: make([]GolspObject, len(object.Elements)),
		MapKeys: make([]GolspObject, len(object.MapKeys)),
		Map: make(map[string]GolspObject),
		Scope: GolspScope{
			Parent: object.Scope.Parent,
			Identifiers: make(map[string]GolspObject),
		},
	}

	for k, o := range object.Scope.Identifiers { newobject.Scope.Identifiers[k] = copyObject(o) }
	for i, e := range object.Elements { newobject.Elements[i] = copyObject(e) }
	for i, k := range object.MapKeys { newobject.MapKeys[i] = copyObject(k) }
	for k, v := range object.Map { newobject.Map[k] = copyObject(v) }

	return newobject
}

// IsolateScope: 'Isolate' a scope object by copying all values from its parent
// scopes into the scope object, effectively orphaning it and flattening its
// inheritance tree
// `scope`: the scope to isolate
// this function returns the isolated scope
func IsolateScope(scope GolspScope) GolspScope {
	newscope := GolspScope{Identifiers: make(map[string]GolspObject)}
	if scope.Parent != nil {
		parent := IsolateScope(*(scope.Parent))
		newscope.Parent = &parent
	}
	for k, o := range scope.Identifiers {
		obj := copyObject(o)
		obj.Scope.Parent = &newscope
		newscope.Identifiers[k] = obj
	}

	return newscope
}

// evalSlice: Evaluate a slice expression, i.e `[list begin end step]`
// `list`: the list or string that is sliced
// `arguments`: the arguments passed in the expression
// this function returns a slice of the list/string or UNDEFINED
func evalSlice(list GolspObject, arguments []GolspObject) GolspObject {
	if len(arguments) == 0 { return list }

	listlen := len(list.Elements)
	if list.Type == GolspObjectTypeLiteral { listlen = len(list.Value.Head) - 2 }

	if len(arguments) == 1 {
		indexf, _ := strconv.ParseFloat(arguments[0].Value.Head, 64)
		index := int(indexf)
		if index < 0 { index += listlen }
		if index < 0 || index >= listlen {
			return Builtins.Identifiers[UNDEFINED]
		}

		if list.Type == GolspObjectTypeList { return list.Elements[index] }

		liststr := []rune(list.Value.Head[1:listlen + 1])
		str := fmt.Sprintf("\"%v\"", string(liststr[index:index + 1]))

		return GolspObject{
			Type: GolspObjectTypeLiteral,
			Value: STNode{
				Type: STNodeTypeStringLiteral,
				Head: str,
			},
		}
	}

	startf, _ := strconv.ParseFloat(arguments[0].Value.Head, 64)
	start := int(startf)
	end := listlen
	step := 1

	if len(arguments) > 2 && arguments[2].Value.Type == STNodeTypeNumberLiteral {
		stepf, _ := strconv.ParseFloat(arguments[2].Value.Head, 64)
		step = int(stepf)
		if step == 0 { return Builtins.Identifiers[UNDEFINED] }
		if step < 0 { end = -listlen - 1 }
	}

	if arguments[1].Value.Type == STNodeTypeNumberLiteral {
		endf, _ := strconv.ParseFloat(arguments[1].Value.Head, 64)
		end = int(endf)
	}

	if start < 0 { start += listlen }
	if end < 0 { end += listlen }

	slice := GolspObject{
		Type: list.Type,
		Elements: make([]GolspObject, 0, listlen),
	}
	slicestr := make([]rune, 0, listlen)
	var liststr []rune
	if list.Type == GolspObjectTypeLiteral {
		liststr = []rune(list.Value.Head[1:listlen + 1])
	}

	if start < 0 || start >= listlen {
		if list.Type == GolspObjectTypeLiteral {
			slice.Value = STNode{
				Type: STNodeTypeStringLiteral,
				Head: fmt.Sprintf("\"%v\"", string(slicestr)),
			}
		}

		return slice
	}

	for i := start; i != end; i += step {
		if i >= listlen { break }
		if i < 0 { break }

		if slice.Type == GolspObjectTypeList {
			slice.Elements = append(slice.Elements, list.Elements[i])
		} else {
			slicestr = append(slicestr, liststr[i])
		}
	}

	if list.Type == GolspObjectTypeLiteral {
		slice.Value = STNode{
			Type: STNodeTypeStringLiteral,
			Head: fmt.Sprintf("\"%v\"", string(slicestr)),
		}
	}

	return slice
}

// evalMap: Lookup key(s) in a map
// `glmap`: the map object
// `arguments`: the key or keys to look up
// this function returns the object or list of objects that the key(s) map to
func evalMap(glmap GolspObject, arguments []GolspObject) GolspObject {
	if len(arguments) == 0 { return glmap }
	if len (arguments) == 1 {
		value, exists := glmap.Map[arguments[0].Value.Head]
		if arguments[0].Type != GolspObjectTypeLiteral || !exists {
			return Builtins.Identifiers[UNDEFINED]
		}
		return value
	}

	values := make([]GolspObject, len(arguments))
	for i, arg := range arguments {
		value, exists := glmap.Map[arg.Value.Head]
		if arg.Type != GolspObjectTypeLiteral || !exists {
			values[i] = Builtins.Identifiers[UNDEFINED]
		} else {
			values[i] = value
		}
	}

	return GolspObject{
		Type: GolspObjectTypeList,
		Elements: values,
	}
}

// SpreadNode: Apply the spread operator to a syntax tree node
// `scope`: the scope within which the node is being spread
// `node`: the node to spread
// this function returns the list of GolspObjects that the node spreads to
func SpreadNode(scope GolspScope, node STNode) []GolspObject {
	nodescope := MakeScope(&scope)
	obj := Eval(nodescope, node)
	if obj.Value.Head == UNDEFINED { return make([]GolspObject, 0) }

	if obj.Type != GolspObjectTypeList &&
		obj.Type != GolspObjectTypeMap &&
		obj.Value.Type != STNodeTypeStringLiteral {
		return []GolspObject{obj}
	}

	if obj.Type == GolspObjectTypeList { return obj.Elements }
	if obj.Type == GolspObjectTypeMap { return obj.MapKeys }

	str := obj.Value.Head[1:len(obj.Value.Head) - 1]
	objects := make([]GolspObject, len(str))

	for i, r := range str {
		objects[i] = GolspObject{
			Type: GolspObjectTypeLiteral,
			Value: STNode{
				Type: STNodeTypeStringLiteral,
				Head: fmt.Sprintf("\"%v\"", string(r)),
			},
		}
	}

	return objects
}

// bindArguments: Bind the arguments passed to a function to the function
// object's Scope property
// `exprhead`: the 'expression head' i.e function object
// `pattern`: the matched pattern, based on which arguments will be bound
// to identifiers
// `argobjects`: the arguments passed to the function that will be bound to
// identifiers
func bindArguments(exprhead GolspObject, pattern []STNode, argobjects []GolspObject) {
	for i, symbol := range pattern {
		if symbol.Type == STNodeTypeStringLiteral || symbol.Type == STNodeTypeNumberLiteral {
			continue
		}

		if symbol.Type == STNodeTypeIdentifier {
			if symbol.Spread {
				exprhead.Scope.Identifiers[symbol.Head] = GolspObject{
					Type: GolspObjectTypeList,
					Elements: argobjects[i:],
				}
				break
			}
			exprhead.Scope.Identifiers[symbol.Head] = argobjects[i]
			continue
		}

		if argobjects[i].Type == GolspObjectTypeList && symbol.Type == STNodeTypeList {
			bindArguments(exprhead, symbol.Children, argobjects[i].Elements)
		}

		if argobjects[i].Type == GolspObjectTypeMap && symbol.Type == STNodeTypeMap {
			// this is a giant mess. clean it up

			mapped := make(map[string]bool)
			mappatternindex := 0
			for iterindex, child := range symbol.Children {
				mappatternindex = iterindex
				if !(child.Type == STNodeTypeNumberLiteral ||
					child.Type == STNodeTypeStringLiteral) {
					break
				}

				if child.Zip == nil { continue }

				value, exists := argobjects[i].Map[child.Head]
				if !exists { continue }

				bindArguments(exprhead, []STNode{*child.Zip}, []GolspObject{value})
				mapped[child.Head] = true
			}

			keys := make([]GolspObject, 0, len(argobjects[i].MapKeys))
			values := make([]GolspObject, 0, len(argobjects[i].MapKeys))
			for _, key := range argobjects[i].MapKeys {
				if !mapped[key.Value.Head] {
					keys = append(keys, key)
					values = append(values, argobjects[i].Map[key.Value.Head])
				}
			}

			patternkeys := symbol.Children[mappatternindex:]
			patternvalues := make([]STNode, 0, len(patternkeys))
			for _, c := range patternkeys {
				if c.Zip == nil { continue }
				patternvalues = append(patternvalues, *c.Zip)
			}

			bindArguments(exprhead, patternkeys, keys)
			bindArguments(exprhead, patternvalues, values)
		}
	}
}

// Eval: Evaluate a syntax tree node within a scope
// `scope`: the scope within which to evaluate the node
// `root`: the root node to evaluate
// this function returns the result of evaluating the node as a GolspObject
func Eval(scope GolspScope, root STNode) GolspObject {
	// root node is a scope -- it evaluates to the result of the last expression
	// in the scope
	// scope nodes are isolated from their parents to ensure that they do not
	// cause side-effects, especially important for 'go' blocks
	if root.Type == STNodeTypeScope {
		newscope := IsolateScope(scope)

		var result GolspObject
		for _, child := range root.Children {
			if child.Spread {
				spread := SpreadNode(newscope, child)
				result = spread[len(spread) - 1]
			} else {
				result = Eval(newscope, child)
			}
		}

		return copyObject(result)
	}

	// string and number literals simply evaluate to themselves
	if root.Type == STNodeTypeNumberLiteral || root.Type == STNodeTypeStringLiteral {
		return GolspObject{
			Type: GolspObjectTypeLiteral,
			Value: root,
		}
	}

	// identifers evaluate to their corresponding values within the scope or UNDEFINED
	if root.Type == STNodeTypeIdentifier {
		return LookupIdentifier(scope, root.Head)
	}

	// 'list' type syntax tree nodes evaluate to 'list' type GolspObjects
	// note that list elements are evaluated immediately, unlike quote expressions
	// in Lisp
	if root.Type == STNodeTypeList {
		elements := make([]GolspObject, 0, len(root.Children))
		for _, c := range root.Children {
			if c.Spread {
				elements = append(elements, SpreadNode(scope, c)...)
			} else {
				elements = append(elements, Eval(MakeScope(&scope), c))
			}
		}

		return GolspObject{
			Type: GolspObjectTypeList,
			Elements: elements,
		}
	}

	// 'map' type syntax tree nodes evaluate to maps
	if root.Type == STNodeTypeMap {
		obj := GolspObject{
			Type: GolspObjectTypeMap,
			Map: make(map[string]GolspObject),
			MapKeys: make([]GolspObject, 0, len(root.Children)),
		}

		for _, c := range root.Children {
			if c.Zip == nil { continue }
			var left []GolspObject
			var right []GolspObject

			if c.Spread {
				left = SpreadNode(scope, c)
			} else {
				left = []GolspObject{Eval(MakeScope(&scope), c)}
			}
			if c.Zip.Spread {
				right = SpreadNode(scope, *c.Zip)
			} else {
				right = []GolspObject{Eval(MakeScope(&scope), *c.Zip)}
			}

			minlen := int(math.Min(float64(len(left)), float64(len(right))))
			for index := 0; index < minlen; index++ {
				if left[index].Type != GolspObjectTypeLiteral {
					continue
				}

				_, exists := obj.Map[left[index].Value.Head]
				obj.Map[left[index].Value.Head] = right[index]
				if !exists {
					obj.MapKeys = append(obj.MapKeys, left[index])
				}
			}
		}

		return obj
	}

	// at this point the root node must be an expression

	// empty expressions evaluate to UNDEFINED
	if len(root.Children) == 0 { return Builtins.Identifiers[UNDEFINED] }

	// exprhead is the head of the expression, aka the function
	// that is being called, list that is being sliced, etc...
	// argobjects is the rest of the expression, the arguments passed
	// to exprhead
	// arguments are evaluated in their own scope (argscope) to prevent side effects
	var exprhead GolspObject
	argobjects := make([]GolspObject, 0, len(root.Children))
	argscope := MakeScope(&scope)

	if root.Children[0].Spread {
		spread := SpreadNode(scope, root.Children[0])
		if len(spread) == 0 { return Builtins.Identifiers[UNDEFINED] }
		exprhead = spread[0]
		argobjects = spread[1:]
	} else {
		exprhead = Eval(MakeScope(&scope), root.Children[0])
	}

	// the function's argument scope is cleared every time it is called
	// since the arguments will be bound again
	if exprhead.Type == GolspObjectTypeFunction {
		exprhead.Scope.Identifiers = make(map[string]GolspObject)
	}

	// evaluating an expression with a number literal or UNDEFINED head
	// produces the literal or UNDEFINED
	// i.e [1 2 3] evals to 1, [undefined a b c] evals to undefined
	if exprhead.Type == GolspObjectTypeLiteral &&
		(exprhead.Value.Type == STNodeTypeNumberLiteral ||
		exprhead.Value.Head == UNDEFINED) {
		return exprhead
	}

	// if exprhead is a list or string literal, slice it
	// if it is a map, lookup key
	if exprhead.Type == GolspObjectTypeList ||
		exprhead.Type == GolspObjectTypeMap ||
		exprhead.Value.Type == STNodeTypeStringLiteral {
		for _, c := range root.Children[1:] {
			if c.Spread {
				argobjects = append(argobjects, SpreadNode(argscope, c)...)
			} else {
				argobjects = append(argobjects, Eval(argscope, c))
			}
		}

		if exprhead.Type == GolspObjectTypeMap {
			return evalMap(exprhead, argobjects)
		}

		return evalSlice(exprhead, argobjects)
	}

	// at this point the expression must be a function call

	fn := exprhead.Function
	builtin := fn.BuiltinFunc != nil

	// builtin functions are called without evaluating the
	// argument syntax tree nodes, these functions can decide how to eval
	// arguments on their own
	if builtin {
		for _, c := range root.Children[1:] {
			obj := GolspObject{
				Type: GolspObjectTypeBuiltinArgument,
				Value: c,
			}
			argobjects = append(argobjects, obj)
		}

		return fn.BuiltinFunc(scope, argobjects)
	}

	// at this point the expression must be a calling a user-defined function
	// all arguments are evaluated immediately, unlike Haskell's lazy evaluation
	for _, c := range root.Children[1:] {
		if c.Spread {
			argobjects = append(argobjects, SpreadNode(argscope, c)...)
		} else {
			argobjects = append(argobjects, Eval(argscope, c))
		}
	}

	patternindex := matchPatterns(fn, argobjects)
	pattern := fn.FunctionPatterns[patternindex]

	// calling a function with fewer arguments than required evaluates to UNDEFINED
	// might possibly implement automatic partial evaluation in the future
	if len(argobjects) < len(pattern) {
		return Builtins.Identifiers[UNDEFINED]
	}

	bindArguments(exprhead, pattern, argobjects)

	return Eval(exprhead.Scope, fn.FunctionBodies[patternindex])
}

// Run: Run a Golsp program
// `program`: the program to run
// `dirname`: the directory of the program file
// this function returns the result of running the program
func Run(dirname string, program string) GolspObject {
	InitializeBuiltins(dirname)
	result := Eval(Builtins, MakeST(Tokenize(program)))
	defer WaitGroup.Wait()

	return result
}
