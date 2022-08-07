package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
)

type OrderInfo struct {
	TimeStr       string  `json:"dateordered"`   // str representing when order was placed
	Asin          string  `json:"asin"`          // str represnting item number of product
	OriginalPrice float32 `json;"originalprice"` // float32 representing the cost the item was bought for
	//TODO: get current price of item currPrice
}

//@param: path string - this string represents the filepath of the csv in question
//@return: Returns an array where each element is an OrderInfo struct containing key item details
func New(path string) []OrderInfo {
	//let's first open our csv
	file, err := os.Open(path)
	if err != nil {
		fmt.Println(err)
	}

	//from the OS file reader, we create a reader for csv
	r := csv.NewReader(file)
	r.FieldsPerRecord = 36 // magic number, oops, but this is how many fields are in our CSV
	orderhist := []OrderInfo{}

	//lets get the headers we care about, noting the first line contains all the field names
	r.Read()

	//now lets read the actual contents of the csv file
	//note, each iteration of the for loop reads one record
	for i := 0; true; i++ {
		//catch errors
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println(err)
		}

		//populate our order info fields and create struct
		s1, s2 := record[0], record[4]
		f1, err := strconv.ParseFloat(record[12][1:], 32)
		orderhist = append(orderhist, OrderInfo{s1, s2, float32(f1)})
	}

	return orderhist
}

func main() {
	b, err := json.Marshal(New("C:\\dev\\GoProjects\\test\\example.csv"))
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(string(b))
}
