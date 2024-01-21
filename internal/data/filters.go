package data

import "greenlight.gustavosantos.net/internal/validator"

type Filters struct {
    Page int
    PageSize int
    Sort string
    SortSafelist []string
}

func ValidateFilters(v *validator.Validator, f Filters) {
    v.Check(f.Page > 0 , "page", "must be greater than zero")
    v.Check(f.Page <= 10_000_000 , "page", "must be a maximum of 10 milion")
    v.Check(f.PageSize > 0 , "page_size", "must be greater than zero")
    v.Check(f.PageSize <= 100 , "page_size", "must be a maximum of 100")
    v.Check(validator.PermittedValue[string](f.Sort, f.SortSafelist...), "sort", "invalid sort value")
}
