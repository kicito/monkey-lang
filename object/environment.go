package object

func NewEnclosedEnvironment(outer *Environment) *Environment {
	env := NewEnvironment()
	env.outer = outer
	return env
}

func NewEnvironment() *Environment {
	s := make(map[string]Object)
	return &Environment{store: s, outer: nil}
}

type Environment struct {
	store map[string]Object
	outer *Environment
}

func (e Environment) Get(name string) (Object, bool) {
	obj, ok := e.store[name]
	if !ok && e.outer != nil {
		obj, ok = e.outer.Get(name)
	}
	return obj, ok
}
func (e *Environment) Set(name string, val Object) Object {
	e.store[name] = val
	return val
}

func (e Environment) ToHash() Object {

	pairs := make(map[HashKey]HashPair)
	for key, valueNode := range e.store {
		keyObj := &String{Value: key}
		hashKey := keyObj.HashKey()

		pairs[hashKey] = HashPair{Key: keyObj, Value: valueNode}
	}

	return &Hash{Pairs: pairs}
}
