// Copyright (c) 2016 10X Genomics, Inc. All rights reserved.
package ligolib

import "fmt"
import "log"

/*
 * This implements a simple mechanism for managing WHERE clauses.
 */

/*
 * There interface for constructing where clauses just needs to be able to
 * return a string describing the clause and handle the special case of
 * an empty clause.
 */

type WhereAble interface {
	Empty() bool
	Stringify() string
}

/*
 * Simple type to describe an empty where clause. I.E. Select everything.
 */
type EmptyWhere struct{}

func (e *EmptyWhere) Empty() bool {
	return true
}

func (e *EmptyWhere) Stringify() string {
	log.Printf("WARNING: Trying to stringify an empty WHERE clause!")
	return "1=1"
}

/*
 * Where clause that is just a string representing the text to insert
 * after the WHERE statement
 */
type StringWhere struct {
	WhereClause string
}

func (w *StringWhere) Empty() bool {
	return w.WhereClause == ""
}

func (w *StringWhere) Stringify() string {
	return w.WhereClause
}

/* Implement a where clause using SQL IN syntax to select from a list */
type ListWhere struct {
	Items  []string
	Column string
}

func (w *ListWhere) Empty() bool {
	return len(w.Items) == 0
}

func (w *ListWhere) Stringify() string {
	result := ""

	for i := 0; i < len(w.Items); i++ {
		if i == 0 {
			result += fmt.Sprintf("'%v'", w.Items[i])
		} else {
			result += fmt.Sprintf(",'%v'", w.Items[i])
		}
	}

	return w.Column + " IN (" + result + ")"
}

func NewListWhere(column string, items []string) *ListWhere {
	return &ListWhere{items, column}

}

/*
 * Some functions to make WhereAble structs.
 */
func NewEmptyWhere() WhereAble {
	return new(EmptyWhere)
}

func NewStringWhere(s string) WhereAble {
	return &StringWhere{s}
}

/*
 * Merge multiple where clauses together.
 */
func MergeWhereClauses(wa ...WhereAble) WhereAble {
	empty := true
	clause := ""

	for _, partial_clause := range wa {
		if !partial_clause.Empty() {
			empty = false
			if clause != "" {
				clause = clause + " AND (" + partial_clause.Stringify() + ")"
			} else {
				clause = "(" + partial_clause.Stringify() + ")"
			}
		}
	}

	if !empty {
		return NewStringWhere(clause)
	} else {
		return NewEmptyWhere()
	}
}

/*
 * Render the SQL for a where clause.
 */
func RenderWhereClause(w WhereAble) string {
	if w.Empty() {
		return ""
	} else {
		return " WHERE " + w.Stringify()
	}
}

func TestWhereable() {
	a := NewStringWhere("a=b")
	b := NewEmptyWhere()

	MergeWhereClauses(a, b)

}
