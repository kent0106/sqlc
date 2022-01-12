# a support in version sqlc: A SQL Compiler

sqlc generates **type-safe code** from SQL. Here's how it works:

1. You write queries in SQL.
1. You run sqlc to generate code with type-safe interfaces to those queries.
1. You write application code that calls the generated code.
1. support in syntax
Check out [an interactive example](https://github.com/xiazemin/sqlc_study) to see it in action.


#安装
 go get -u github.com/xiazemin/sqlc
 go get -u github.com/xiazemin/sqlc/cmd/sqlc
#使用
实例：https://github.com/xiazemin/sqlc_study

## Sponsors

sqlc development is funded by our [generous
sponsors](https://github.com/sponsors/xiazemin), including the following
companies:

If you use sqlc at your company, please consider [becoming a
sponsor](https://github.com/sponsors/xiazemin) today.

Sponsors receive priority support via the sqlc Slack organization.

支持驼峰格式
支持生成mock代码

 //go:generate  mockgen -source=./querier.go  -destination=./mock/querier.go

 go generate ./... 

 解决聚合函数返回值是interface{}
 必须用下面函数解析的问题
 ```
 func ParseInt64(v interface{}) (int64, error) {
	if v == nil {
		return 0, nil
	}
	raw, ok := v.([]uint8)
	if !ok {
		return 0, fmt.Errorf("type assert failed")
	}
	return strconv.ParseInt(string(raw), 10, 64)
}
```

//解决null 问题
sql: Scan error on column index 0, name "sum(size)": converting NULL to int64 is unsupported

问题1:
如果字段定义了default null 返回的是sql.Nullxxx

问题2：
如果 字段定义了 NOT NULL，但是没有命中记录
返回null怎么处理，怎么扫描？
所以安全起见 应该都是返回sql.Nullxxx



使用聚合函数的情况下
如果NOT NULL，没有命中会返回 NULL ,且走了索引才会，in 不会
             命中了 返回默认值
如果DEFAULT NULL，但是没有插入数据，且没有命中会返回NULL  用了sql.null不会报错
                            命中了 返回NULL
              如果插入数据了，没有命中会返回NULL
                            命中了 返回NULL

//安全起见
1，规范写ifnull 0 
2，聚合函数都返回sqlNullxxx    √  这个更安全，拿到0值更符合预期


//修复bug
如果 有两个IN ，第一个参数长度是 1 时候，会把第二个参数替换到第一个位置
IN (?) AND cond IN (?)
应该被替换成
IN (?) AND cond IN (?,?,?)
实际替换成
IN (?,?,?) AND cond IN (?)


//枚举支持中文，default null
A const declaration gives names to constants, that is, values that are fixed at compile time. The value of a constant must be a number, string, or boolean.
const的值只能是数字，字符串或者布尔值。所以不能生成sql.NullString