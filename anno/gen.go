package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type InterfaceSpec struct {
	Path   string
	Name   string
	Auth   string
	Method string
}
type ControllerSpec struct {
	Path       string
	Name       string
	Interfaces []*InterfaceSpec
}

type AuthKV struct {
	Url    string `yaml:"url"`
	Permit string `yaml:"permit"`
}

type PermitSpec struct {
	Authentications []*AuthKV `yaml:"permits"`
	WhiteList       []string  `yaml:"white_list"`
}

var ptName = regexp.MustCompile(`name="(.*?)"`)
var ptFunc = regexp.MustCompile(`method="(.*?)"`)
var ptPath = regexp.MustCompile(`path=("/.*?")`)
var ptAuth = regexp.MustCompile(`auth=(".*?")`)
var ptOpLog = regexp.MustCompile(`opLog=(".*?")`)

func main() {
	fset := token.NewFileSet()
	var ctrls []*ControllerSpec
	authSpec := &PermitSpec{}
	err := filepath.WalkDir("../server", func(path string, d fs.DirEntry, err error) error {
		if !d.Type().IsDir() {
			if strings.HasPrefix(d.Name(), "server_") {
				f, err := os.Open(path)
				if err != nil {
					return err
				}
				codeBytes, err := io.ReadAll(f)
				if err != nil {
					return err
				}
				astf, err := parser.ParseFile(fset, "", string(codeBytes), parser.ParseComments|parser.AllErrors)
				if err != nil {
					fmt.Printf("err = %s", err)
				}
				controller := ControllerSpec{}
				ast.Inspect(astf, func(n ast.Node) bool {
					switch t := n.(type) {
					case *ast.FuncDecl:
						doc := strings.Trim(t.Doc.Text(), "\t \n")
						if doc == "" || !strings.HasPrefix(doc, "go:interface") {
							return true
						}
						inter := parseInterface(t.Name.String(), doc, &controller, authSpec)
						controller.Interfaces = append(controller.Interfaces, inter)
					case *ast.File:
						doc := strings.Trim(t.Doc.Text(), "\t \n")
						if doc == "" || !strings.HasPrefix(doc, "go:controller") {
							return true
						}
						parseController(doc, &controller)
					}
					return true
				})
				ctrls = append(ctrls, &controller)

			}
		}
		return nil
	})
	if err != nil {
		return
	}
	genRouter(ctrls)
	ymlBytes, err := yaml.Marshal(authSpec)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	nf, err := os.OpenFile("../conf/permit.yml", os.O_CREATE|os.O_TRUNC|os.O_APPEND|os.O_RDWR, os.ModePerm)
	if err != nil {
		return
	}
	_, err = nf.Write(ymlBytes)
	if err != nil {
		return
	}
	err = nf.Sync()
	if err != nil {
		return
	}
	err = nf.Close()
	if err != nil {
		return
	}
}

func parseController(doc string, ctrl *ControllerSpec) {
	strName := ptName.FindStringSubmatch(doc)
	strPath := ptPath.FindStringSubmatch(doc)
	if len(strName) > 1 {
		ctrl.Name = strName[1]
	}
	if len(strPath) > 1 {
		ctrl.Path = strPath[1]
	}
}

func parseInterface(funcName string, doc string, ctrl *ControllerSpec, auth *PermitSpec) *InterfaceSpec {
	strFunc := ptFunc.FindStringSubmatch(doc)
	strPath := ptPath.FindStringSubmatch(doc)
	strAuth := ptAuth.FindStringSubmatch(doc)
	strLog := ptOpLog.FindStringSubmatch(doc)
	inter := &InterfaceSpec{Name: funcName}
	if len(strFunc) > 1 && len(strPath) > 1 {
		inter.Method = strings.ToLower(strFunc[1])
		inter.Method = strings.ToUpper(inter.Method[:1]) + inter.Method[1:]
		inter.Path = strPath[1]
		urlPath := strings.Trim(ctrl.Path, "\"") + strings.Trim(inter.Path, "\"")
		if len(strAuth) > 1 {
			inter.Auth = strings.Trim(strAuth[1], "\"")
			kv := &AuthKV{
				Url:    urlPath,
				Permit: inter.Auth,
			}
			auth.Authentications = append(auth.Authentications, kv)
			if len(strLog) > 1 {
				kv.Permit = inter.Auth + "|" + strings.Trim(strLog[1], "\"")
			}
		} else {
			if len(strLog) > 1 {
				auth.WhiteList = append(auth.WhiteList, urlPath+"|"+strings.Trim(strLog[1], "\""))
			} else {
				auth.WhiteList = append(auth.WhiteList, urlPath)
			}

		}
		return inter
	}
	return nil
}

var routerTemplate = `
package server

`

func genRouter(ctrls []*ControllerSpec) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", routerTemplate, parser.ParseComments)
	if err != nil {
		panic(err)
	}
	f.Decls = append(f.Decls, &ast.GenDecl{
		Tok: token.IMPORT,
		Specs: []ast.Spec{
			&ast.ImportSpec{
				Path: &ast.BasicLit{
					Kind:  token.STRING,
					Value: "\"github.com/gofiber/fiber/v2\"",
				},
			},
		},
	})
	rootAst := addRootFuncDecl(ctrls)
	for _, ctrl := range ctrls {
		f.Decls = append(f.Decls, addRouterGroupFunc(ctrl.Name, ctrl.Interfaces))
		rootAst.Body.List = append(rootAst.Body.List, &ast.ExprStmt{ //表达式语句
			X: &ast.CallExpr{
				Fun:    ast.NewIdent("srv." + ctrl.Name + "Register"),
				Lparen: 0,
				Args: []ast.Expr{
					&ast.BasicLit{
						Kind:  token.DEFAULT,
						Value: ctrl.Name,
					},
				},
				Ellipsis: 0,
				Rparen:   0,
			},
		})
	}
	f.Decls = append(f.Decls, rootAst)

	var buf bytes.Buffer
	if err := format.Node(&buf, fset, f); err != nil {
		panic(err)
	}
	genFile("../server/router.go", buf)
}

func addRootFuncDecl(ctrls []*ControllerSpec) *ast.FuncDecl {
	funcDelc := &ast.FuncDecl{
		Name: ast.NewIdent("Register"),
		Body: &ast.BlockStmt{
			List: []ast.Stmt{},
		},
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{
						{
							Name: "srv",
							Obj:  ast.NewObj(ast.Var, "srv"),
						},
					},
					Type: &ast.StarExpr{
						X: ast.NewIdent("AdminServer"),
					},
				},
			},
		},
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					{
						Names: []*ast.Ident{
							{
								Name: "root",
								Obj:  ast.NewObj(ast.Var, "root"),
							},
						},
						Type: &ast.SelectorExpr{
							X:   ast.NewIdent("fiber"),
							Sel: ast.NewIdent("Router"),
						},
					},
				},
			},
		},
	}
	for _, ctrl := range ctrls {
		funcDelc.Body.List = append(funcDelc.Body.List, &ast.AssignStmt{ //表达式语句
			Lhs: []ast.Expr{ast.NewIdent(ctrl.Name)},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{
				&ast.CallExpr{
					Fun:    ast.NewIdent("root.Group"),
					Lparen: 0,
					Args: []ast.Expr{
						&ast.BasicLit{
							Kind:  token.STRING,
							Value: ctrl.Path,
						},
					},
					Ellipsis: 0,
					Rparen:   0,
				},
			},
		})
	}
	return funcDelc
}

func addRouterGroupFunc(name string, ints []*InterfaceSpec) *ast.FuncDecl {
	funcDelc := &ast.FuncDecl{
		Name: ast.NewIdent(name + "Register"),
		Body: &ast.BlockStmt{
			List: []ast.Stmt{},
		},
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{
						{
							Name: "srv",
							Obj:  ast.NewObj(ast.Var, "srv"),
						},
					},
					Type: &ast.StarExpr{
						X: ast.NewIdent("AdminServer"),
					},
				},
			},
		},
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					{
						Names: []*ast.Ident{
							{
								Name: "root",
								Obj:  ast.NewObj(ast.Var, "root"),
							},
						},
						Type: &ast.SelectorExpr{
							X:   ast.NewIdent("fiber"),
							Sel: ast.NewIdent("Router"),
						},
					},
				},
			},
		},
	}
	for _, inter := range ints {
		funcDelc.Body.List = append(funcDelc.Body.List, &ast.ExprStmt{ //表达式语句
			X: &ast.CallExpr{
				Fun:    ast.NewIdent("root." + inter.Method),
				Lparen: 0,
				Args: []ast.Expr{
					&ast.BasicLit{
						Kind:  token.STRING,
						Value: inter.Path,
					},
					&ast.BasicLit{
						Kind:  token.DEFAULT,
						Value: "srv." + inter.Name,
					},
				},
				Ellipsis: 0,
				Rparen:   0,
			},
		})
	}
	return funcDelc
}

func genFile(fileName string, buf bytes.Buffer) {
	nf, err := os.OpenFile(fileName, os.O_CREATE|os.O_TRUNC|os.O_APPEND|os.O_RDWR, os.ModePerm)
	if err != nil {
		return
	}
	nf.Write(buf.Bytes())
	nf.Sync()
	nf.Close()
	cmd := fmt.Sprintf("go fmt %s;", fileName)
	runCmd("/bin/sh", "-c", cmd)
}
func runCmd(name string, args ...string) {
	cmd := exec.Command(name, args...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	defer stderr.Close()
	if err = cmd.Start(); err != nil {
		fmt.Println(err.Error())
		return
	}
	opBytes, err := io.ReadAll(stderr)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	fmt.Println(string(opBytes))
}
