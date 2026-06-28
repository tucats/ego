package resources

import "strings"

// Sort creates a sort order for the handle for subsequent
// read operations. The parameters are the column names used to
// sort the data. If the list is empty, there is no sort order
// to be used.
func (r *ResHandle) Sort(names ...string) *ResHandle {
	if r == nil {
		return nil
	}

	r.OrderList = nil

	if len(names) > 0 {
		r.OrderList = []int{}
		for _, name := range names {
			for index, column := range r.Columns {
				if strings.EqualFold(name, column.Name) {
					r.OrderList = append(r.OrderList, index)
				}
			}
		}
	}

	return r
}

// GenerateOrder creates a SQL phrase that defines the sort order
// of a query. If there is no active list, this returns an empty
// string.
func (r ResHandle) OrderBy() string {
	b := strings.Builder{}

	if len(r.OrderList) > 0 {
		for index, column := range r.OrderList {
			if index == 0 {
				b.WriteString(" order by ")
			} else {
				b.WriteString(", ")
			}

			b.WriteString(r.Columns[column].SQLName)
		}
	}

	return b.String()
}
