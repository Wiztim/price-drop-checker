package main

import (
	"encoding/csv"
	//"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
)

type ItemCategories struct {
	PriceReduced   []OrderInfo `json:"priceReduced"`
	PriceUnchanged []OrderInfo `json:"priceUnchanged"`
	PriceIncreased []OrderInfo `json:"princeIncreased"`
	Unavailable    []OrderInfo `json:"unavailable"`
}

type OrderInfo struct {
	Name          string  `json:"name"`
	TimeStr       string  `json:"dateOrdered"`   // str representing when order was placed
	Asin          string  `json:"asin"`          // str represnting item number of product
	OriginalPrice float64 `json:"originalPrice"` // float representing the cost the item was bought for
	CurrentPrice  float64 `json:"currentPrice"`  // curr price
	PriceDrop     float64 `json:"priceDrop"`     // calced as originalPrice - currentPrice - positive number indicates a drop, negative number indicates an increase
	//Returnable	  bool	  `json:"returnable"`	 // TODO!!
	//PARSE https://www.amazon.com/spr/returns/cart?ref_=orc_gift&orderId= [insert order id here]
	// SEARCH BAR ON THIS PAGE https://www.amazon.com/gp/css/returns/homepage.html?ref_=footer_hy_f_4
}

type myMessage struct {
	plstr string `json:"message"`
}

var (
	// ErrNameNotProvided is thrown when a name is not provided
	ErrNameNotProvided = errors.New("no name was provided in the HTTP body")
)

// Handler is Lambda function handler
func Handler(request myMessage) (ItemCategories, error) {
	fmt.Println("This should be the body: ", request.plstr)
	// If no name is provided in the HTTP request body, throw an error
	if len(request.plstr) < 1 {
		return ItemCategories{}, ErrNameNotProvided
	}

	return New(request.plstr)
}

// @param: path string - this string represents the filepath of the csv in question
// @return: Returns an array where each element is an OrderInfo struct containing key item details
func New(body string) (ItemCategories, error) {
	//parse info from csv
	orderhist := parseCSV(body)

	//this gets info for each item from web request
	//TODO: add parallelization
	getoriginalprice(&orderhist)

	//let's now categorize each listing
	result := categorizeItems(&orderhist)
	return result, nil
}

// populates our array with info from csv
func parseCSV(body string) []OrderInfo {
	//reading csv given by frontend in body of POST
	file := strings.NewReader(body)

	//from the str file reader, we create a reader for csv
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

		//parsing date like a bitch
		firstdiv := strings.Index(record[0], "/")
		month := record[0][0:firstdiv]
		if firstdiv < 2 {
			month = "0" + month
		}

		str := record[0][firstdiv+1:]
		seconddiv := strings.Index(str, "/")
		day := str[:seconddiv]
		if seconddiv < 2 {
			day = "0" + day
		}

		year := str[seconddiv+1:]

		timestamp, err := time.Parse("01/02 03:04:05PM '06 -0700", month+"/"+day+" 00:00:00AM '"+year[2:]+" -0000")
		if err != nil {
			fmt.Println(err)
		}

		if time.Duration.Hours(time.Now().Sub(timestamp)) > 24*30 {
			//fmt.Println("over 30 days")
			continue
		}

		//populate our order info fields and create struct
		s0, s1, s2 := record[2], record[0], record[4]

		//now we just parse the float, removing any whitespace from the string
		f1, err := strconv.ParseFloat(strings.TrimSpace(record[12][1:]), 64)
		if err != nil {
			fmt.Println(err)
		}

		//we can now mostly make an OrderInfo object
		orderhist = append(orderhist, OrderInfo{s0, s1, s2, f1, 0, 0})
	}

	return orderhist
}

// populates currente price and price drop in orderhist array
// uses sync.WaitGroup
func getoriginalprice(orderhist *[]OrderInfo) {
	var wg sync.WaitGroup
	for i := range *orderhist {
		wg.Add(1) //we are starting a goroutine
		go getOriginalPricePerItem(&((*orderhist)[i]), &wg)
	}
	//let's wait for all goroutines to finish
	wg.Wait()
}

func getOriginalPricePerItem(item *OrderInfo, wg *sync.WaitGroup) {
	defer wg.Done() //when this function ends, a goroutine will finish, so let's decrement the counter.

	//webscraping portion!
	//we first generate the URL we need to GET
	itemUrl := getUrl(item.Name, item.Asin)

	//now let's actually GET the webpage
	resp, err := http.Get(itemUrl)
	if err != nil {
		fmt.Println(err)
	}

	//let's look at the HTML body of the response
	body, err := io.ReadAll(resp.Body)
	strbody := string(body)

	//we first check if the item is unavailable, because if it is unavailable, we will get some NOT okay prices.
	//this tag should only appear on unavailable items
	if priceindex := strings.Index(strbody, `<span class="a-color-price a-text-bold">Currently unavailable.</span>`); priceindex != -1 {
		//unavailable, so let's just skip skip the next call
		//fmt.Println("Sorry, but, ", (*orderhist)[i].Name, " is currently unavailable.")
		return
	}

	//note: the first instance of `<span aria-hidden="true">$` seems to be the actual price of the item before tax, but this
	//is entirely empiracally decided
	//NEW IN USE: THIS IS BETTER??? <span class="a-offscreen">$
	priceindex := strings.Index(strbody, `<span class="a-offscreen">$`)

	//now we need to get the number in this string right after the priceindex
	//we know this number ends because the span is terminated with "<"
	//also note `<span class="a-offscreen">$` is 27 chars long
	var strresult string = ""
	for strbody[priceindex+27] != '<' {
		strresult += string(strbody[priceindex+27])
		priceindex++
	}
	price, err := strconv.ParseFloat(strresult, 64)
	if err != nil {
		fmt.Println(err)
		//fmt.Println("Cannot get price from: ", getUrl((*orderhist)[i].Name, (*orderhist)[i].Asin))
	}

	//now lets actually modify our item
	item.CurrentPrice = price
	item.PriceDrop = math.Round((item.OriginalPrice-price)*100) / 100

	//done!
	resp.Body.Close()
	//fmt.Println("Price obtained from URL: ", getUrl((*orderhist)[i].Name, (*orderhist)[i].Asin), " is: ", price)
}

func getUrl(name, asin string) string {
	//this structure is gotten from analyzing enough URLs, the name part can be literally anything, but this is what I chose
	itemurl := "https://www.amazon.com/" + name[:15] + "/dp/" + asin + "/"
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
		case x == (*orderhist)[i].CurrentPrice: //this means it is unavailable
			cat.Unavailable = append(cat.Unavailable, (*orderhist)[i])
		case x == 0: //no drop
			cat.PriceUnchanged = append(cat.PriceUnchanged, (*orderhist)[i])
		case x < 0: //price increased
			cat.PriceIncreased = append(cat.PriceIncreased, (*orderhist)[i])
		}
	}

	return cat
}

func main() {
	lambda.Start(Handler)
}
