package main

import (
	"compress/flate"
	"compress/gzip"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rs/cors"
)

type PriceChangeCategories struct {
	PriceReduced   []OrderInfo `json:"priceReduced"`
	PriceUnchanged []OrderInfo `json:"priceUnchanged"`
	PriceIncreased []OrderInfo `json:"priceIncreased"`
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
	NAME_COLUMN           = 3
	DATE_ORDERED_COLUMN   = 1
	ASIN_COLUMN           = 5
	ORIGINAL_PRICE_COLUMN = 4
	CAPTCHA_INDICATOR     = `For information about migrating to our APIs refer to our Marketplace APIs at https://developer.amazonservices.com/ref=rm_c_sv, or our Product Advertising API at https://affiliate-program.amazon.com/gp/advertising/api/detail/main.html/ref=rm_c_ac for advertising use cases.`
	UNAVAILABLE_INDICATOR = `<span class="a-color-price a-text-bold">Currently unavailable.</span>`
	PRICE_INDICATOR       = `<span class="a-offscreen">$`
)

// Handler is Lambda function handler
func Handler(request string) (PriceChangeCategories, error) {
	//note: we must return a valid PriceChangeCategories struct, it cannot be nil
	categories, err := newPriceChangeCategories(request)
	if err != nil {
		fmt.Println(err)
		return PriceChangeCategories{}, err
	}
	return categories, nil
}

func newPriceChangeCategories(body string) (PriceChangeCategories, error) {
	orderHistory, err := parseCSV(body)
	if err != nil {
		return PriceChangeCategories{}, err
	}

	//this gets info for each item from web request
	getPriceInfo(&orderHistory)

	//let's now categorize each listing
	result := categorizeItems(&orderHistory)
	return result, nil
}

// populates a new slice with info from csv
func parseCSV(requestBody string) ([]OrderInfo, error) {
	csvReader, err := validateCSV(requestBody)
	if err != nil {
		return nil, err
	}

	//create the return object to be filled
	orderHistory := []OrderInfo{}

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

// validateCSV returns a csv reader of the string when the string is a proper CSV containing the headers we need.
func validateCSV(requestBody string) (*csv.Reader, error) {
	//we should not have received an empty request.
	if len(requestBody) < 1 {
		return nil, errors.New("validateCSV: Request body empty.")
	}
	//we first correctly format the input for reading
	requestBody = strings.Replace(requestBody, `\n`, "\n", -1)

	//reading csv given by frontend in body of POST request
	stringReader := strings.NewReader(requestBody)
	csvReader := csv.NewReader(stringReader)

	//Let's make sure we have the headers we care about: Title (Name), Date Ordered, ASIN/ISBN (Asin), and Purchase Price Per Unit (OriginalPrice)
	csvHeaders, err := csvReader.Read()
	if err != nil {
		return nil, errors.New("validateCSV: could not read first" + err.Error())
	}
	if csvHeaders[NAME_COLUMN] != "description" || csvHeaders[DATE_ORDERED_COLUMN] != "order date" || csvHeaders[ASIN_COLUMN] != "ASIN" || csvHeaders[ORIGINAL_PRICE_COLUMN] != "price" {
		return nil, errors.New(`validateCSV: Missing "description", "Date Ordered", "ASIN", or "price" fields in CSV`)
	}
	return csvReader, nil
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
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	x := 503
	y := 502
	for i := range *orderHistory {
		randomNumber := (int(r.Uint32()) % x) + y
		getPriceInfoForItem(&(*orderHistory)[i])
		time.Sleep(time.Duration(randomNumber))
	}
}

// if there is an error, we will just return early.
// we want to keep getting prices. Errored items will get put into the unavailable category
func getPriceInfoForItem(item *OrderInfo) {
	//webscraping portion!
	//we first generate the URL we need to GET
	itemUrl := getUrl(item.Asin)

	// Create a new GET request
	req, err := http.NewRequest("GET", itemUrl, nil)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}

	// Set the required headers
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Cache-Control", "no-cache")
	//req.Header.Set("Cookie", "TODO: use env variable")
	req.Header.Set("Sec-Ch-Ua", `"Chromium";v="116", "Not)A;Brand";v="24", "Google Chrome";v="116"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"Windows"`)
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/116.0.0.0 Safari/537.36")

	// Send the request and get the response
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		return
	}

	// Process the response here
	fmt.Println("Response status code:", resp.StatusCode)

	if err != nil {
		fmt.Println("getPriceInfoForItem: Cannot retrieve webpage for " + itemUrl + err.Error())
		return
	}
	if resp.Status != "200 OK" {
		fmt.Println("Received HTTP status " + resp.Status + " when retrieving " + itemUrl)
		return
	}

	// Decompress the response body if it's compressed
	var reader io.Reader
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err = gzip.NewReader(resp.Body)
		if err != nil {
			fmt.Println("Error creating gzip reader:", err)
			return
		}
	case "deflate":
		reader = flate.NewReader(resp.Body)
	default:
		reader = resp.Body
	}

	// Read the response body
	body, err := io.ReadAll(reader)
	if err != nil {
		fmt.Println("getPriceInfoForItem: Cannot read body of response from", itemUrl, err.Error())
		return
	}

	// Convert the body to a string
	respBodyString := string(body)
	defer resp.Body.Close()

	//We first check if we got served a captcha - the webpage will only have the following quote if the program was served a captcha
	//This is exceedingly common by the way.
	if strings.Index(respBodyString, CAPTCHA_INDICATOR) != -1 {
		fmt.Println("getPriceInfoForItem: Got served a captcha by Amazon for making non-API automated requests. Occured for item: " + item.Name)
		return
	}

	//we first check if the item is unavailable, because if it is unavailable, we will get some NOT okay prices.
	//this tag should only appear on unavailable items
	if strings.Index(respBodyString, UNAVAILABLE_INDICATOR) != -1 {
		//unavailable, so let's just skip this item
		fmt.Println("The item " + item.Name + " is listed as unavailable.")
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

func getUrl(asin string) string {
	//this structure is gotten from analyzing enough URLs, the name part can be literally anything, but this is what I chose
	itemurl := "https://www.amazon.com/" + "dp/" + asin + "/"
	return itemurl
}

func categorizeItems(orderHistory *[]OrderInfo) PriceChangeCategories {
	categories := PriceChangeCategories{}

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

func handlePost(w http.ResponseWriter, r *http.Request) {
	b, err := io.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}

	//fmt.Println(Handler(string(b)))
	resp, err := Handler((string(b)))
	if err != nil {
		fmt.Println("Error sending request to server")
		return
	}

	fmt.Println(resp)
	responseJSON, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, "Failed to marshal JSON", http.StatusInternalServerError)
		return
	}
	//w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")

	w.Write(responseJSON)
}

func main() {
	//this is necessary for the Lambda instance
	//lambda.Start(Handler)
	//this is useful for local testing
	//fmt.Println(Handler("Order Date,Order ID,Title,Category,ASIN/ISBN,UNSPSC Code,Website,Release Date,Condition,Seller,Seller Credentials,List Price Per Unit,Purchase Price Per Unit,Quantity,Payment Instrument Type,Purchase Order Number,PO Line Number,Ordering Customer Email,Shipment Date,Shipping Address Name,Shipping Address Street 1,Shipping Address Street 2,Shipping Address City,Shipping Address State,Shipping Address Zip,Order Status,Carrier Name & Tracking Number,Item Subtotal,Item Subtotal Tax,Item Total,Tax Exemption Applied,Tax Exemption Type,Exemption Opt-Out,Buyer Name,Currency,Group Name\n\n10/01/22,111-8005663-4090615,\"SKYN Original Condoms, 24 Count (Pack of 1)\",CONDOM,\"B004TTXA7I\",\"53131622\",Amazon.com,,new,Amazon.com,,$20.99,$11.17,1,\"Discover0179\",,,noreply@gmail.com,06/02/20,Noah Terminello,2235 MANDRILL AVE,,VENTURA,CA,93003-7014,Shipped,AMZN_US(TBA050996544001),$11.17,$0.87,$12.04,,,,Noah Terminello,USD,"))

	mux := http.NewServeMux()
	mux.HandleFunc("/", handlePost)

	// Create a CORS handler with the desired CORS options
	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"}, // Replace with your frontend URL
		AllowedMethods: []string{"POST"},
	})

	// Use the CORS middleware with your HTTP server
	handler := c.Handler(mux)

	//http.HandleFunc("/post", handlePost)
	/*
		http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

		// Serve the main HTML page
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, "static/index.html")
		})
	*/
	// Start the server on port 8080
	http.ListenAndServe(":8080", handler)

}
