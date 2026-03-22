// demo.tengo - examples of scripting builtins

pr("program:", kush.name)
pr("program path:", kush.path)
pr("args:", args)

// base64 encode/decode
s := "hello world"
e := encode64(s)
pr("encode64:", e)
pr("decode64:", decode64(e))

u := encode64url(s)

pr("encode64url:", u)
pr("decode64url:", decode64url(u))

// numeric base enc/dec (encoden/decoden)
// encoden expects decimal string id as first arg, optional size as second arg
id := "123456789"
encN := encoden(id, 37)   // encode into base-37
pr("encoden (base37):", encN)
pr("decoden (base37):", decoden(encN, 37))

// loaders: pass a path as the first positional arg to demo.tengo to test
path := ""
if len(args) > 0 {
    path = args[0]
}

if path == "" {
    pr("loadfile/loadtext: no path argument provided; to test, run: kush demo.tengo /path/to/file")
} else {
    b := loadfile(path)
    pr("loadfile length (bytes):", len(b))
    t := loadtext(path)
    if t == "" {
        pr("loadtext: empty (error reading file or not UTF-8)")
    } else {
		enc:= encoden(t, 42)
		pr("loadtext length (chars):", len(t))
		pr("encoden (base42):", enc)
		pr("decoden (base42):", decoden(enc, 42))
    }
}
