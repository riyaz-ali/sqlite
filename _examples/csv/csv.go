package main

import (
	"encoding/csv"
	"fmt"
	"go.riyazali.net/sqlite"
	"io"
	"os"
	"strconv"
	"strings"
)

// CsvModule provides an implementation of an sqlite virtual table for reading CSV files.
// It's based on the original sqlite csv module here https://www.sqlite.org/src/artifact?filename=ext/misc/csv.c
// It follows the same interface as defined in sqlite docs at https://sqlite.org/csv.html
type CsvModule struct{}

// Parameters:
//    filename=FILENAME          Name of file containing CSV content
//    header=YES|NO              First row of CSV defines the names of
//                               columns if "yes".  Default "no".
func (c *CsvModule) Connect(_ *sqlite.Conn, args []string, declare func(string) error) (_ sqlite.VirtualTable, err error) {
	args = args[2:]
	var table = &CsvVirtualTable{}
	var readHeader = false

	for i := range args {
		s := strings.SplitN(args[i], "=", 2)
		fmt.Println(s)
		switch s[0] {
		case "file":
			table.file, err = strconv.Unquote(s[1])
			if err != nil {
				return nil, err
			}
		case "header":
			readHeader = s[1] == "yes"
		}
	}

	file, err := os.Open(table.file)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	row, err := csv.NewReader(file).Read()
	if err != nil {
		return nil, err
	}
	table.columns = len(row)

	var d string
	if readHeader {
		table.skipHeader = true
		d = strings.Join(row, ",")
	} else {
		var a []string
		for i := 0; i < len(row); i++ {
			a = append(a, fmt.Sprintf("c%d", i))
		}
		d = strings.Join(row, ",")
	}
	return table, declare(fmt.Sprintf("CREATE TABLE x(%s)", d))
}

// same as Connect
func (c *CsvModule) Create(conn *sqlite.Conn, args []string, declare func(string) error) (sqlite.VirtualTable, error) {
	return c.Connect(conn, args, declare)
}

// CsvVirtualTable is an instance of the csv virtual table. It is bound to a single csv file / data.
type CsvVirtualTable struct {
	file       string // pointer to an open file reader
	columns    int    // number of column in the csv file
	skipHeader bool
}

func (c *CsvVirtualTable) BestIndex(_ *sqlite.IndexInfoInput) (*sqlite.IndexInfoOutput, error) {
	return &sqlite.IndexInfoOutput{EstimatedCost: 1000000}, nil
}

func (c *CsvVirtualTable) Open() (_ sqlite.VirtualCursor, err error) {
	file, err := os.Open(c.file)
	if err != nil {
		return nil, err
	}
	var reader = csv.NewReader(file)
	if c.skipHeader {
		if _, err = reader.Read(); err != nil {
			return nil, err
		}
	}

	return &CsvCursor{
		closer:  file,
		csv:     reader,
		current: nil,
		rowid:   -1,
	}, nil
}

func (c *CsvVirtualTable) Disconnect() error { return nil }
func (c *CsvVirtualTable) Destroy() error    { return c.Disconnect() }

// CsvCursor is an instance of the csv file cursor. Only a full table scan is supported natively.
type CsvCursor struct {
	closer  io.Closer   // closes the input to csv.Reader
	csv     *csv.Reader // csv reader / parser
	current []string    // current row that the cursor points to
	rowid   int64       // current rowid .. negative for EOF
}

func (c *CsvCursor) Next() error {
	record, err := c.csv.Read()
	if err != nil && err != io.EOF {
		return err
	} else if err == io.EOF {
		c.rowid = -1
		return sqlite.SQLITE_OK
	}

	c.rowid += 1
	c.current = record
	return sqlite.SQLITE_OK
}

func (c *CsvCursor) Column(ctx *sqlite.Context, i int) error {
	if i >= 0 && i < len(c.current) && c.current[i] != "" {
		ctx.ResultText(c.current[i])
	}
	return nil
}

func (c *CsvCursor) Filter(int, string, ...sqlite.Value) error { c.rowid = 0; return c.Next() }
func (c *CsvCursor) Rowid() (int64, error)                     { return c.rowid, nil }
func (c *CsvCursor) Eof() bool                                 { return c.rowid < 0 }
func (c *CsvCursor) Close() error                              { return c.closer.Close() }

func init() {
	sqlite.Register(func(api *sqlite.ExtensionApi) (sqlite.ErrorCode, error) {
		if err := api.CreateModule("csv", &CsvModule{}); err != nil {
			return sqlite.SQLITE_ERROR, err
		}
		return sqlite.SQLITE_OK, nil
	})
}

func main() {}
