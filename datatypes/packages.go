package datatypes

// @tomcole this should be made a first-class object
// with name, type, and values map.
type EgoPackage map[string]interface{}

func (p *EgoPackage) String() string {
	return Format(p)
}
