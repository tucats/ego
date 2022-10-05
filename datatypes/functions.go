package datatypes

type FunctionParameter struct {
	Name     string
	ParmType Type
}

type FunctionDeclaration struct {
	Name        string
	Parameters  []FunctionParameter
	ReturnTypes []Type
}
