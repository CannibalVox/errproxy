package errproxy

type ErrorTransformer func(error) error
