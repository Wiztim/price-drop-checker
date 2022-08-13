package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
)

type ItemCategories struct {
	PriceReduced   []OrderInfo `json:"priceReduced"`
	PriceUnchanged []OrderInfo `json:"priceUnchanged"`
	PriceIncreased []OrderInfo `json:"princeIncreased"`
}

type OrderInfo struct {
	Name          string  `json:"name"`
	TimeStr       string  `json:"dateOrdered"`   // str representing when order was placed
	Asin          string  `json:"asin"`          // str represnting item number of product
	OriginalPrice float64 `json:"originalPrice"` // float32 representing the cost the item was bought for
	CurrentPrice  float64 `json:"currentPrice"`  //TODO: get current price of item currPrice
	PriceDrop     float64 `json:"priceDrop"`     // calced as originalPrice - currentPrice - positive number indicates a drop, negative number indicates an increase
}

//@param: path string - this string represents the filepath of the csv in question
//@return: Returns an array where each element is an OrderInfo struct containing key item details
func New(path string) ItemCategories {
	//parse info from csv
	orderhist := parseCSV(path)

	//this gets info for each item from web request
	//TODO: add parallelization
	getoriginalprice(&orderhist)

	//let's now categorize each listing
	result := categorizeItems(&orderhist)
	return result
}

//populates our array with info from csv
func parseCSV(path string) []OrderInfo {
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
		s0, s1, s2 := record[2], record[0], record[4]

		//let's do a little bit of editing on the name
		//if there is a comma in the name, it indicates quantities of the package. We want to just remove this part of the name
		//by taking a substring of the name until we get to the comma
		if x := strings.Index(s0, ","); x > -1 {
			s0 = s0[:x]
		}

		//now we just parse the float
		f1, err := strconv.ParseFloat(record[12][1:], 64)
		if err != nil {
			fmt.Println(err)
		}

		//we can now mostly make an OrderInfo object
		orderhist = append(orderhist, OrderInfo{s0, s1, s2, f1, 0, 0})
	}

	return orderhist
}

//populates currente price in orderhist array
func getoriginalprice(orderhist *[]OrderInfo) {
	for i := range *orderhist {
		//webscraping portion!
		//we first generate the URL we need to GET
		itemurl := getUrl((*orderhist)[i].Name, (*orderhist)[i].Asin)

		//now let's actually GET the webpage
		resp, err := http.Get(itemurl)
		if err != nil {
			fmt.Println(err)
		}

		//let's look at the HTML body of the response
		body, err := io.ReadAll(resp.Body)
		strbody := string(body)

		//note: the first instance of `<span aria-hidden="true">$` seems to be the actual price of the item before tax, but this
		//is entirely empiracally decided
		priceindex := strings.Index(strbody, `<span aria-hidden="true">$`)

		//now we need to get the number in this string right after the priceindex
		//we know this number ends because the span is terminated with "<"
		//also note `<span aria-hidden="true">$` is 26 chars long
		var strresult string = ""
		for strbody[priceindex+26] != '<' {
			strresult += string(strbody[priceindex+26])
			priceindex++
		}
		price, err := strconv.ParseFloat(strresult, 64)
		if err != nil {
			fmt.Println(err)
		}

		//now lets actually modify our array
		(*orderhist)[i].CurrentPrice = price
		(*orderhist)[i].PriceDrop = math.Round(((*orderhist)[i].OriginalPrice-price)*100) / 100

		//done!
	}
}

func getUrl(name, asin string) string {
	//this structure is gotten from analyzing enough URLs
	itemurl := "https://www.amazon.com/" + name + "/dp/" + asin + "/"
	return itemurl
}

func categorizeItems(orderhist *[]OrderInfo) ItemCategories {
	cat := ItemCategories{}

	// if we sort the orderhist items by price, they will come out sorted when we put them in the categories
	// so we will just sort them by pricedrop now
	sort.Slice((*orderhist), func(i, j int) bool {
		return (*orderhist)[i].PriceDrop > (*orderhist)[j].PriceDrop
	})

	for i := range *orderhist {
		switch x := (*orderhist)[i].PriceDrop; {
		case x > 0: //aka price did drop
			cat.PriceReduced = append(cat.PriceReduced, (*orderhist)[i])
		case x == 0: //no drop
			cat.PriceUnchanged = append(cat.PriceReduced, (*orderhist)[i])
		case x < 0: //price increased
			cat.PriceIncreased = append(cat.PriceReduced, (*orderhist)[i])
		}
	}

	return cat
}

func main() {
	b, err := json.Marshal(New("C:\\dev\\GoProjects\\test\\example.csv"))
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(string(b))
}
