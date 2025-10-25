package db

import (
	"context"
	"errors"
	"io"

	"github.com/go-pg/pg/v10"
	"github.com/go-pg/pg/v10/orm"
)

type CommonRepo struct {
	db      orm.DB
	filters map[string][]Filter
	sort    map[string][]SortField
	join    map[string][]string
}

// NewCommonRepo returns new repository
func NewCommonRepo(db orm.DB) CommonRepo {
	return CommonRepo{
		db: db,
		filters: map[string][]Filter{
			Tables.User.Name:     {StatusFilter},
			Tables.Category.Name: {StatusFilter},
			Tables.Expense.Name:  {StatusFilter},
		},
		sort: map[string][]SortField{
			Tables.User.Name:     {{Column: Columns.User.CreatedAt, Direction: SortDesc}},
			Tables.Category.Name: {{Column: Columns.Category.CreatedAt, Direction: SortDesc}},
			Tables.Expense.Name:  {{Column: Columns.Expense.CreatedAt, Direction: SortDesc}},
		},
		join: map[string][]string{
			Tables.User.Name:     {TableColumns},
			Tables.Category.Name: {TableColumns, Columns.Category.User},
			Tables.Expense.Name:  {TableColumns, Columns.Expense.User, Columns.Expense.Category},
		},
	}
}

// WithTransaction is a function that wraps CommonRepo with pg.Tx transaction.
func (cr CommonRepo) WithTransaction(tx *pg.Tx) CommonRepo {
	cr.db = tx
	return cr
}

// WithEnabledOnly is a function that adds "statusId"=1 as base filter.
func (cr CommonRepo) WithEnabledOnly() CommonRepo {
	f := make(map[string][]Filter, len(cr.filters))
	for i := range cr.filters {
		f[i] = make([]Filter, len(cr.filters[i]))
		copy(f[i], cr.filters[i])
		f[i] = append(f[i], StatusEnabledFilter)
	}
	cr.filters = f

	return cr
}

/*** User ***/

// FullUser returns full joins with all columns
func (cr CommonRepo) FullUser() OpFunc {
	return WithColumns(cr.join[Tables.User.Name]...)
}

// DefaultUserSort returns default sort.
func (cr CommonRepo) DefaultUserSort() OpFunc {
	return WithSort(cr.sort[Tables.User.Name]...)
}

// UserByID is a function that returns User by ID(s) or nil.
func (cr CommonRepo) UserByID(ctx context.Context, id int, ops ...OpFunc) (*User, error) {
	return cr.OneUser(ctx, &UserSearch{ID: &id}, ops...)
}

// OneUser is a function that returns one User by filters. It could return pg.ErrMultiRows.
func (cr CommonRepo) OneUser(ctx context.Context, search *UserSearch, ops ...OpFunc) (*User, error) {
	obj := &User{}
	err := buildQuery(ctx, cr.db, obj, search, cr.filters[Tables.User.Name], PagerTwo, ops...).Select()

	if errors.Is(err, pg.ErrMultiRows) {
		return nil, err
	} else if errors.Is(err, pg.ErrNoRows) || errors.Is(err, io.EOF) {
		return nil, nil
	}

	return obj, err
}

// UsersByFilters returns User list.
func (cr CommonRepo) UsersByFilters(ctx context.Context, search *UserSearch, pager Pager, ops ...OpFunc) (users []User, err error) {
	err = buildQuery(ctx, cr.db, &users, search, cr.filters[Tables.User.Name], pager, ops...).Select()
	return
}

// CountUsers returns count
func (cr CommonRepo) CountUsers(ctx context.Context, search *UserSearch, ops ...OpFunc) (int, error) {
	return buildQuery(ctx, cr.db, &User{}, search, cr.filters[Tables.User.Name], PagerOne, ops...).Count()
}

// AddUser adds User to DB.
func (cr CommonRepo) AddUser(ctx context.Context, user *User, ops ...OpFunc) (*User, error) {
	q := cr.db.ModelContext(ctx, user)
	if len(ops) == 0 {
		q = q.ExcludeColumn(Columns.User.CreatedAt)
	}
	applyOps(q, ops...)
	_, err := q.Insert()

	return user, err
}

// UpdateUser updates User in DB.
func (cr CommonRepo) UpdateUser(ctx context.Context, user *User, ops ...OpFunc) (bool, error) {
	q := cr.db.ModelContext(ctx, user).WherePK()
	if len(ops) == 0 {
		q = q.ExcludeColumn(Columns.User.CreatedAt)
	}
	applyOps(q, ops...)
	res, err := q.Update()
	if err != nil {
		return false, err
	}

	return res.RowsAffected() > 0, err
}

// DeleteUser set statusId to deleted in DB.
func (cr CommonRepo) DeleteUser(ctx context.Context, id int) (deleted bool, err error) {
	user := &User{ID: id, StatusID: StatusDeleted}

	return cr.UpdateUser(ctx, user, WithColumns(Columns.User.StatusID))
}

/*** Category ***/

// FullCategory returns full joins with all columns
func (cr CommonRepo) FullCategory() OpFunc {
	return WithColumns(cr.join[Tables.Category.Name]...)
}

// DefaultCategorySort returns default sort.
func (cr CommonRepo) DefaultCategorySort() OpFunc {
	return WithSort(cr.sort[Tables.Category.Name]...)
}

// CategoryByID is a function that returns Category by ID(s) or nil.
func (cr CommonRepo) CategoryByID(ctx context.Context, id int, ops ...OpFunc) (*Category, error) {
	return cr.OneCategory(ctx, &CategorySearch{ID: &id}, ops...)
}

// OneCategory is a function that returns one Category by filters. It could return pg.ErrMultiRows.
func (cr CommonRepo) OneCategory(ctx context.Context, search *CategorySearch, ops ...OpFunc) (*Category, error) {
	obj := &Category{}
	err := buildQuery(ctx, cr.db, obj, search, cr.filters[Tables.Category.Name], PagerTwo, ops...).Select()

	if errors.Is(err, pg.ErrMultiRows) {
		return nil, err
	} else if errors.Is(err, pg.ErrNoRows) || errors.Is(err, io.EOF) {
		return nil, nil
	}

	return obj, err
}

// CategoriesByFilters returns Category list.
func (cr CommonRepo) CategoriesByFilters(ctx context.Context, search *CategorySearch, pager Pager, ops ...OpFunc) (categories []Category, err error) {
	err = buildQuery(ctx, cr.db, &categories, search, cr.filters[Tables.Category.Name], pager, ops...).Select()
	return
}

// CountCategories returns count
func (cr CommonRepo) CountCategories(ctx context.Context, search *CategorySearch, ops ...OpFunc) (int, error) {
	return buildQuery(ctx, cr.db, &Category{}, search, cr.filters[Tables.Category.Name], PagerOne, ops...).Count()
}

// AddCategory adds Category to DB.
func (cr CommonRepo) AddCategory(ctx context.Context, category *Category, ops ...OpFunc) (*Category, error) {
	q := cr.db.ModelContext(ctx, category)
	if len(ops) == 0 {
		q = q.ExcludeColumn(Columns.Category.CreatedAt)
	}
	applyOps(q, ops...)
	_, err := q.Insert()

	return category, err
}

// UpdateCategory updates Category in DB.
func (cr CommonRepo) UpdateCategory(ctx context.Context, category *Category, ops ...OpFunc) (bool, error) {
	q := cr.db.ModelContext(ctx, category).WherePK()
	if len(ops) == 0 {
		q = q.ExcludeColumn(Columns.Category.ID, Columns.Category.CreatedAt)
	}
	applyOps(q, ops...)
	res, err := q.Update()
	if err != nil {
		return false, err
	}

	return res.RowsAffected() > 0, err
}

// DeleteCategory set statusId to deleted in DB.
func (cr CommonRepo) DeleteCategory(ctx context.Context, id int) (deleted bool, err error) {
	category := &Category{ID: id, StatusID: StatusDeleted}

	return cr.UpdateCategory(ctx, category, WithColumns(Columns.Category.StatusID))
}

/*** Expense ***/

// FullExpense returns full joins with all columns
func (cr CommonRepo) FullExpense() OpFunc {
	return WithColumns(cr.join[Tables.Expense.Name]...)
}

// DefaultExpenseSort returns default sort.
func (cr CommonRepo) DefaultExpenseSort() OpFunc {
	return WithSort(cr.sort[Tables.Expense.Name]...)
}

// ExpenseByID is a function that returns Expense by ID(s) or nil.
func (cr CommonRepo) ExpenseByID(ctx context.Context, id int, ops ...OpFunc) (*Expense, error) {
	return cr.OneExpense(ctx, &ExpenseSearch{ID: &id}, ops...)
}

// OneExpense is a function that returns one Expense by filters. It could return pg.ErrMultiRows.
func (cr CommonRepo) OneExpense(ctx context.Context, search *ExpenseSearch, ops ...OpFunc) (*Expense, error) {
	obj := &Expense{}
	err := buildQuery(ctx, cr.db, obj, search, cr.filters[Tables.Expense.Name], PagerTwo, ops...).Select()

	if errors.Is(err, pg.ErrMultiRows) {
		return nil, err
	} else if errors.Is(err, pg.ErrNoRows) || errors.Is(err, io.EOF) {
		return nil, nil
	}

	return obj, err
}

// ExpensesByFilters returns Expense list.
func (cr CommonRepo) ExpensesByFilters(ctx context.Context, search *ExpenseSearch, pager Pager, ops ...OpFunc) (expenses []Expense, err error) {
	err = buildQuery(ctx, cr.db, &expenses, search, cr.filters[Tables.Expense.Name], pager, ops...).Select()
	return
}

// CountExpenses returns count
func (cr CommonRepo) CountExpenses(ctx context.Context, search *ExpenseSearch, ops ...OpFunc) (int, error) {
	return buildQuery(ctx, cr.db, &Expense{}, search, cr.filters[Tables.Expense.Name], PagerOne, ops...).Count()
}

// AddExpense adds Expense to DB.
func (cr CommonRepo) AddExpense(ctx context.Context, expense *Expense, ops ...OpFunc) (*Expense, error) {
	q := cr.db.ModelContext(ctx, expense)
	if len(ops) == 0 {
		q = q.ExcludeColumn(Columns.Expense.CreatedAt)
	}
	applyOps(q, ops...)
	_, err := q.Insert()

	return expense, err
}

// UpdateExpense updates Expense in DB.
func (cr CommonRepo) UpdateExpense(ctx context.Context, expense *Expense, ops ...OpFunc) (bool, error) {
	q := cr.db.ModelContext(ctx, expense).WherePK()
	if len(ops) == 0 {
		q = q.ExcludeColumn(Columns.Expense.ID, Columns.Expense.CreatedAt)
	}
	applyOps(q, ops...)
	res, err := q.Update()
	if err != nil {
		return false, err
	}

	return res.RowsAffected() > 0, err
}

// DeleteExpense set statusId to deleted in DB.
func (cr CommonRepo) DeleteExpense(ctx context.Context, id int) (deleted bool, err error) {
	expense := &Expense{ID: id, StatusID: StatusDeleted}

	return cr.UpdateExpense(ctx, expense, WithColumns(Columns.Expense.StatusID))
}
