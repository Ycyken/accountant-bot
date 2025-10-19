package saldo

import "saldo/pkg/db"

//go:generate colgen -imports=saldo/pkg/db
//colgen:User
//colgen:User:MapP(db.User)
//colgen:Category
//colgen:Category:MapP(db.Category)
//colgen:Expense
//colgen:Expense:MapP(db.Expense)

type User struct {
	db.User
}

func NewUser(in *db.User) *User {
	if in == nil {
		return nil
	}

	return &User{
		User: *in,
	}
}

type Category struct {
	db.Category
}

func NewCategory(in *db.Category) *Category {
	if in == nil {
		return nil
	}

	return &Category{
		Category: *in,
	}
}

type Expense struct {
	db.Expense
}

func NewExpense(in *db.Expense) *Expense {
	if in == nil {
		return nil
	}

	return &Expense{
		Expense: *in,
	}
}

// MapP converts slice of type T to slice of type M with given converter with pointers.
func MapP[T, M any](a []T, f func(*T) *M) []M {
	n := make([]M, len(a))
	for i := range a {
		n[i] = *f(&a[i])
	}
	return n
}
