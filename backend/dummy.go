package main

import (
	"encoding/csv"
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
	DateOrdered   string  `json:"dateOrdered"`   // str representing when order was placed
	Asin          string  `json:"asin"`          // str represnting item number of product
	OriginalPrice float64 `json:"originalPrice"` // float representing the cost the item was bought for
	CurrentPrice  float64 `json:"currentPrice"`  // curr price
	PriceDrop     float64 `json:"priceDrop"`     // calced as originalPrice - currentPrice - positive number indicates a drop, negative number indicates an increase
	//Returnable	  bool	  `json:"returnable"`	 // TODO!!
	//PARSE https://www.amazon.com/spr/returns/cart?ref_=orc_gift&orderId= [insert order id here]
	// SEARCH BAR ON THIS PAGE https://www.amazon.com/gp/css/returns/homepage.html?ref_=footer_hy_f_4
}

const (
	FIELDS_PER_RECORD     = 36
	NAME_COLUMN           = 2
	DATE_ORDERED_COLUMN   = 0
	ASIN_COLUMN           = 4
	ORIGINAL_PRICE_COLUMN = 12
)

// Handler is Lambda function handler
func Handler(request string) (ItemCategories, error) {
	//note: we must return a valid ItemCategories struct, it cannot be nil
	// emptyRequest is thrown when the body of the Lambda Request is empty
	if len(request) < 1 {
		return ItemCategories{}, errors.New("Handler: Bad Request - Empty body")
	}
	return newItemCategories(request)
}

func newItemCategories(body string) (ItemCategories, error) {
	orderHistory, err := parseCSV(body)
	if err != nil {
		return ItemCategories{}, nil
	}

	//this gets info for each item from web request
	getPriceInfo(&orderHistory)

	//let's now categorize each listing
	result := categorizeItems(&orderHistory)
	return result, nil
}

// populates a new slice with info from csv
func parseCSV(requestBody string) ([]OrderInfo, error) {
	//we first correctly format the input for reading
	strings.Replace(requestBody, `\n`, "\n", -1)

	//reading csv given by frontend in body of POST request
	stringReader := strings.NewReader(requestBody)
	csvReader := csv.NewReader(stringReader)

	csvReader.FieldsPerRecord = FIELDS_PER_RECORD
	orderHistory := make([]OrderInfo, 0)

	//Let's make sure we have the headers we care about: Title (Name), Date Ordered, ASIN/ISBN (Asin), and Purchase Price Per Unit (OriginalPrice)
	csvHeaders, err := csvReader.Read()
	if err != nil {
		return nil, errors.New("parseCSV: " + fmt.Sprint(err))
	}
	if csvHeaders[NAME_COLUMN] != "Title" || csvHeaders[DATE_ORDERED_COLUMN] != "Date Ordered" || csvHeaders[ASIN_COLUMN] != "ASIN/ISBN" || csvHeaders[ORIGINAL_PRICE_COLUMN] != "Purchase Price Per Unit" {
		return nil, errors.New(`parseCSV: Missing "Title", "Date Ordered", "ASIN/ISBN", or "Purchase Price Per Unit" fields in CSV`)
	}

	//now lets read the actual contents of the csv file
	//note, each iteration of the for loop reads one record
	for i := 0; true; i++ {
		//catch errors
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		//parsing date to see if item is returnable or not
		orderDate := record[0]
		timestamp, err := time.Parse("01/02/03", orderDate)
		if err != nil {
			return nil, errors.New("parseCSV: " + fmt.Sprint(err))
		}

		//this if statement will activate when the item is within 30 days of being ordered (in other words, the item is in the return window.)
		//This is here mostly as a safeguard should we need to implment it. For now, we trust users to upload CSVs with a somewhat recent order window.
		if time.Duration.Hours(time.Now().Sub(timestamp)) > 24*30 {
		}

		//populate our order info fields and create struct
		itemName, itemDateOrdered, itemAsin := record[NAME_COLUMN], record[DATE_ORDERED_COLUMN], record[ASIN_COLUMN]

		//now we just parse the float, removing any whitespace from the string
		itemOriginalPrice, err := strconv.ParseFloat(strings.TrimSpace(record[ORIGINAL_PRICE_COLUMN][1:]), 64)
		if err != nil {
			return nil, errors.New("parseCSV: could not parse original price")
		}

		//we can now mostly make an OrderInfo object. Unassigned fields will have value of -1
		orderHistory = append(orderHistory, OrderInfo{
			Name:          itemName,
			DateOrdered:   itemDateOrdered,
			Asin:          itemAsin,
			OriginalPrice: itemOriginalPrice,
			CurrentPrice:  -1,
			PriceDrop:     -1})
	}

	return orderHistory, nil
}

// populates currente price and price drop in orderHistory slice
// TODO: Implement errgroup instead of waitgroup
func getPriceInfo(orderHistory *[]OrderInfo) {
	var wg sync.WaitGroup
	for _, item := range *orderHistory {
		//we are starting a goroutine
		wg.Add(1)
		go getPriceInfoForItem(&item, &wg)
	}

	//let's wait for all goroutines to finish
	wg.Wait()
}

func getPriceInfoForItem(item *OrderInfo, wg *sync.WaitGroup) {
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

func categorizeItems(orderHistory *[]OrderInfo) ItemCategories {
	categories := ItemCategories{}

	// if we sort the orderHistory items by price, they will come out sorted when we put them in the categories
	// so we will just sort them by pricedrop now
	sort.Slice((*orderHistory), func(i, j int) bool {
		return (*orderHistory)[i].PriceDrop > (*orderHistory)[j].PriceDrop
	})

	for _, item := range *orderHistory {
		switch x := item.PriceDrop; {
		case x > 0: //aka price did drop
			categories.PriceReduced = append(categories.PriceReduced, item)
		case x == item.CurrentPrice: //this means it is unavailable
			categories.Unavailable = append(categories.Unavailable, item)
		case x == 0: //no drop
			categories.PriceUnchanged = append(categories.PriceUnchanged, item)
		case x < 0: //price increased
			categories.PriceIncreased = append(categories.PriceIncreased, item)
		}
	}

	return categories
}

func main() {
	lambda.Start(Handler)
}
