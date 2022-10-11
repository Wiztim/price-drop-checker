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
	NAME_COLUMN           = 2
	DATE_ORDERED_COLUMN   = 0
	ASIN_COLUMN           = 4
	ORIGINAL_PRICE_COLUMN = 12
	CAPTCHA_INDICATOR     = `For information about migrating to our APIs refer to our Marketplace APIs at https://developer.amazonservices.com/ref=rm_c_sv, or our Product Advertising API at https://affiliate-program.amazon.com/gp/advertising/api/detail/main.html/ref=rm_c_ac for advertising use cases.`
	UNAVAILABLE_INDICATOR = `<span class="a-color-price a-text-bold">Currently unavailable.</span>`
	PRICE_INDICATOR       = `<span class="a-offscreen">$`
)

// Handler is Lambda function handler
func Handler(request string) (ItemCategories, error) {
	//note: we must return a valid ItemCategories struct, it cannot be nil
	categories, err := newItemCategories(request)
	if err != nil {
		fmt.Println(err)
		return ItemCategories{}, err
	}
	return categories, nil
}

func newItemCategories(body string) (ItemCategories, error) {
	orderHistory, err := parseCSV(body)
	if err != nil {
		return ItemCategories{}, err
	}

	//this gets info for each item from web request
	getPriceInfo(&orderHistory)

	//let's now categorize each listing
	result := categorizeItems(&orderHistory)
	return result, nil
}

// populates a new slice with info from csv
func parseCSV(requestBody string) ([]OrderInfo, error) {
	//we should not have received an empty request.
	if len(requestBody) < 1 {
		return nil, errors.New("parseCSV: Request body empty.")
	}

	//we first correctly format the input for reading
	requestBody = strings.Replace(requestBody, `\n`, "\n", -1)

	//reading csv given by frontend in body of POST request
	stringReader := strings.NewReader(requestBody)
	csvReader := csv.NewReader(stringReader)

	orderHistory := make([]OrderInfo, 0)

	//Let's make sure we have the headers we care about: Title (Name), Date Ordered, ASIN/ISBN (Asin), and Purchase Price Per Unit (OriginalPrice)
	csvHeaders, err := csvReader.Read()
	if err != nil {
		return nil, errors.New("parseCSV: could not read first" + err.Error())
	}
	if csvHeaders[NAME_COLUMN] != "Title" || csvHeaders[DATE_ORDERED_COLUMN] != "Order Date" || csvHeaders[ASIN_COLUMN] != "ASIN/ISBN" || csvHeaders[ORIGINAL_PRICE_COLUMN] != "Purchase Price Per Unit" {
		return nil, errors.New(`parseCSV: Missing "Title", "Date Ordered", "ASIN/ISBN", or "Purchase Price Per Unit" fields in CSV`)
	}

	//Note: we currently do not call isItemOrderedWithin30Days(record[DAT_ORDERED_COLUMN]). We trust customers to upload a csv with a recent enough date range
	//But if we did, we would call it right here.

	//now lets read the actual contents of the csv file
	//note, each iteration of the for loop reads one record
	for i := 0; true; i++ {
		//catch errors
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.New("parseCSV: could not read record. " + err.Error())
		}

		//populate our order info fields and create struct
		itemName, itemDateOrdered, itemAsin := record[NAME_COLUMN], record[DATE_ORDERED_COLUMN], record[ASIN_COLUMN]

		//now we just parse the float, removing whitespace and a "$" from the string
		itemOriginalPrice, err := strconv.ParseFloat(strings.TrimSpace(record[ORIGINAL_PRICE_COLUMN][1:]), 64)
		if err != nil {
			return nil, errors.New("parseCSV: could not parse original price" + err.Error())
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

func isItemOrderedWithin30Days(orderDate string) (bool, error) {
	//parsing date to see if item is with 30 day return window or not
	timeStringArr := strings.Split(orderDate, "/")
	if len(timeStringArr[0]) == 1 {
		timeStringArr[0] = "0" + timeStringArr[0]
	}
	if len(timeStringArr[1]) == 1 {
		timeStringArr[1] = "0" + timeStringArr[1]
	}
	timeStringArr[2] = timeStringArr[2][2:]
	formattedTime := timeStringArr[0] + "/" + timeStringArr[1] + "/" + timeStringArr[2]

	timestamp, err := time.Parse("01/02/03", formattedTime)
	if err != nil {
		return false, errors.New("parseCSV: " + err.Error())
	}

	//this if statement will activate when the item is within 30 days of being ordered (in other words, the item is in the return window.)
	if time.Duration.Hours(time.Now().Sub(timestamp)) > 24*30 {
		return false, nil
	}
	return true, nil
}

// populates currente price and price drop in orderHistory slice
func getPriceInfo(orderHistory *[]OrderInfo) {
	var wg sync.WaitGroup
	for _, item := range *orderHistory {
		//we are starting a new goroutine. We call Add(1) for the WaitGroup to track that there is an open thread
		wg.Add(1)
		go func() {
			//decrement the weight group counter once getPriceInfoForItem completes
			defer wg.Done()
			getPriceInfoForItem(&item)
		}()
	}
	//wait for all threads to complete
	wg.Wait()
}

// if there is an error, we will just return early.
// we want to keep getting prices. Errored items will get put into the unavailable category
func getPriceInfoForItem(item *OrderInfo) {
	//webscraping portion!
	//we first generate the URL we need to GET
	itemUrl := getUrl(item.Name, item.Asin)

	//now let's actually GET the webpage
	resp, err := http.Get(itemUrl)
	if err != nil {

		fmt.Println("getPriceInfoForItem: Cannot retrieve webpage for " + itemUrl + err.Error())
		return
	}
	if resp.Status != "200 OK" {
		fmt.Println("Received HTTP status " + resp.Status + " when retrieving " + itemUrl)
		return
	}

	//let's look at the HTML body of the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("getPriceInfoForItem: Cannot ready body of response from " + itemUrl + err.Error())
		return
	}
	respBodyString := string(body)
	defer resp.Body.Close()

	//We first check if we got served a captcha - the webpage will only have the following quote if the program was served a captcha
	//This is exceedingly common by the way.
	if strings.Index(respBodyString, CAPTCHA_INDICATOR) != -1 {
		fmt.Println("getPriceInfoForItem: Got served a captcha by Amazon, likely for suspicious behavior regarding item: " + item.Name)
		return
	}

	//we first check if the item is unavailable, because if it is unavailable, we will get some NOT okay prices.
	//this tag should only appear on unavailable items
	if strings.Index(respBodyString, UNAVAILABLE_INDICATOR) != -1 {
		//unavailable, so let's just skip this item
		fmt.Println("The item " + item.Name[:15] + " is listed as unavailable.")
		return
	}

	//note: the first instance of `<span class="a-offscreen">$` seems to be the actual price of the item before tax, but this is entirely empiracally decided
	priceIndex := strings.Index(respBodyString, PRICE_INDICATOR)

	//now we need to get the number in this string right after the priceindex
	//we know this number ends because the span is terminated with "<"
	//also note `<span class="a-offscreen">$` is 27 chars long
	flagLength := 27
	var priceString string = ""
	for respBodyString[priceIndex+flagLength] != '<' {
		priceString += string(respBodyString[priceIndex+flagLength])
		priceIndex++
	}
	price, err := strconv.ParseFloat(priceString, 64)
	if err != nil {
		fmt.Println("Cannot get price from: " + itemUrl + err.Error())
		return
	}

	//now lets actually modify our item
	item.CurrentPrice = price
	item.PriceDrop = math.Round((item.OriginalPrice-price)*100) / 100
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
		case item.CurrentPrice == -1: //this means it is unavailable since we never set CurrentPrice
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
