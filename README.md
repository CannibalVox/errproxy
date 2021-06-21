# errproxy

CLI to generate wrappers that allow output errors to be transformed

## How to Use

#### Install ProxyWrapper

```bash
go get github.com/CannibalVox/errproxy/proxywrapper
go install github.com/CannibalVox/errproxy/proxywrapper
```

#### Generate the wrappers of Your Choice

See the example/ folder for some examples that use `//go:generate`!

```bash
proxywrapper -input database/sql -type DB -output ./dbwrapper
```

#### Create a wrapper, and use it in place of your target type!

```golang
func connectToDB(driver string, connString string) (*sqlwrapper.SqlDB, error) {
	//Open DB connection
	db, err := sql.Open(driver, connString)
	if err != nil {
		return nil, err
	}

	// Wrap it with an error transformer
	dbWrap := sqlwrapper.WrapSqlDB(db, func(err error) error {
		// Pick out non-5xx codes and give them appropriate status codes
		pgErr, ok := err.(*pq.Error)
		if ok {
			if pgErr.Code == "23505" {
				return stacktrace.PropagateWithCode(err, 
					stacktrace.ErrorCode(codes.AlreadyExists), "unique constraint violation")
			}
		}
		return err
	})

	// Use the same interface as before!  The transformer is propagated to all objects
    // returned from all methods
	rows, err := dbWrap.Query("SELECT value FROM table LIMIT 20")
	if err != nil {
		return nil, err
	}

	defer func() {
		log.Println(rows.Close())
	}()

	for rows.Next() {
		var value int
		err := rows.Scan(&value)
		if err != nil {
			return dbWrap, err
		}

		log.Println("Value: %d", value)
	}

	return dbWrap, nil
}
```

## OK, But Why?

I have strong beliefs about how errors should be handled.  Unless you have some local-specific error handling behavior, they should return to the user surface by default, the error should have enough information on it to determine proper handling, and the user surface (HTTP, GRPC, UI, whatever) should provide that handling.  

There's a lot of great options for attaching semantic data to errors- [palantir/stacktrace](https://github.com/palantir/stacktrace) provides ErrorCodes, GRPC status codes are great, and at a past employer, I created an error library that combined GRPC status codes and a user-facing string.  Regardless of how you choose to add semantic data to your errors, the biggest problem is going to be how to handle interfacing with libraries that don't follow your error doctrine.  Especially complicated libraries, such as database/sql and go-redis.  Auto-generating proxy wrappers is my idea of how to handle that, so here's the tool to do it.

