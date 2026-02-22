// Package phpengine provides CGO bindings to embedded PHP.
//
// This package wraps libphp to execute PHP scripts directly from Go
// without spawning separate processes. It supports PHP versions 7.4-8.4.
//
// The Engine type provides the main interface for PHP execution:
//
//	engine, err := phpengine.NewEngine("8.3")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer engine.Shutdown()
//
//	ctx := phpengine.NewContext(req, "/var/www", "public/index.php")
//	resp, err := engine.Execute(ctx, "public/index.php")
package phpengine
