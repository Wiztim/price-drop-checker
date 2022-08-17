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
}

//@param: path string - this string represents the filepath of the csv in question
//@return: Returns an array where each element is an OrderInfo struct containing key item details
func New() ItemCategories {
	//parse info from csv
	orderhist := parseCSV(
	`Order Date,Order ID,Title,Category,ASIN/ISBN,UNSPSC Code,Website,Release Date,Condition,Seller,Seller Credentials,List Price Per Unit,Purchase Price Per Unit,Quantity,Payment Instrument Type,Purchase Order Number,PO Line Number,Ordering Customer Email,Shipment Date,Shipping Address Name,Shipping Address Street 1,Shipping Address Street 2,Shipping Address City,Shipping Address State,Shipping Address Zip,Order Status,Carrier Name & Tracking Number,Item Subtotal,Item Subtotal Tax,Item Total,Tax Exemption Applied,Tax Exemption Type,Exemption Opt-Out,Buyer Name,Currency,Group Name
7/8/2022,113-1904590-5333829,Amazon Basics 2 Pack CR1632 3 Volt Lithium Coin Cell Battery,BATTERY,B07JLN1WXT,26111700,Amazon.com,,new,Amazon.com,,$6.29 ,$6.29 ,1,Discover7733,,,salgadoguadalupe14@yahoo.com,7/8/2022,Guadalupe Salgado,254 W BARNETT ST,,VENTURA,CA,93001-1614,Shipped,AMZN_US(TBA163049550404),$6.29 ,$0.49 ,$6.78 ,FALSE,,FALSE,Guadalupe,USD,
7/9/2022,113-8370767-1548225,Corduroy Bags Cross body Bag Purse for Women Mini Travel Bags Handbags Eco Bag,HANDBAG,B09KRKT738,53121600,Amazon.com,,new,Eflying Lion,,$19.99 ,$15.99 ,1,MasterCard - 5527 and Gift Certificate/Card,,,salgadoguadalupe14@yahoo.com,7/10/2022,Guadalupe Salgado,254 W BARNETT ST,,VENTURA,CA,93001-1614,Shipped,AMZN_US(TBA164499033604),$15.99 ,$1.24 ,$17.23 ,FALSE,,FALSE,Guadalupe,USD,
7/28/2022,113-6152711-0397020,"PetAmi Dog Treat Pouch | Dog Training Pouch Bag with Waist Shoulder Strap, Poop Bag Dispenser and Collapsible Bowl | Treat Training Bag for Treats, Kibbles, Pet Toys | 3 Ways to Wear (Red)",PET_SUPPLIES,B07GX7T2MW,10111300,Amazon.com,,new,Caravan Group,,$30.99 ,$15.99 ,1,Discover7733,,,salgadoguadalupe14@yahoo.com,7/29/2022,Guadalupe Salgado,254 W BARNETT ST,,VENTURA,CA,93001-1614,Shipped,AMZN_US(TBA191883679304),$15.99 ,$1.24 ,$17.23 ,FALSE,,FALSE,Guadalupe,USD,
7/28/2022,113-6152711-0397020,PATPET Dog Training Collar with Remote - Rechargeable Waterproof Training Collar for Small Medium Large Dogs 3 Training Modes Up to 1000Ft Remote Range,ANIMAL_COLLAR,B09Q91SNL2,10111300,Amazon.com,,new,PaiTevoL,,$39.99 ,$31.99 ,1,Discover7733,,,salgadoguadalupe14@yahoo.com,7/28/2022,Guadalupe Salgado,254 W BARNETT ST,,VENTURA,CA,93001-1614,Shipped,AMZN_US(TBA191037690704),$31.99 ,$2.48 ,$34.47 ,FALSE,,FALSE,Guadalupe,USD,
7/28/2022,113-6152711-0397020,"Dog Muzzle, Soft Muzzle with Geometric Print Pattern for Small Medium Large Dogs Chihuahua Labrador, Adjustable Velcro Muzzle to Stop Biting and Chewing",ANIMAL_MUZZLE,B09W9Q4YH9,10000000,Amazon.com,,new,Crazy Felix,,$22.99 ,$10.99 ,1,Discover7733,,,salgadoguadalupe14@yahoo.com,7/28/2022,Guadalupe Salgado,254 W BARNETT ST,,VENTURA,CA,93001-1614,Shipped,AMZN_US(TBA191259408004),$10.99 ,$0.81 ,$11.80 ,FALSE,,FALSE,Guadalupe,USD,
7/28/2022,113-6152711-0397020,"Nylon Dog Muzzle for Small Medium Large Dogs, Air Mesh Breathable and Drinkable Pet Muzzle for Anti-Biting Anti-Barking Licking (S, Grey)",ANIMAL_MUZZLE,B07QNVP8K1,10141610,Amazon.com,,new,PettyCart,,$15.99 ,$12.99 ,1,Discover7733,,,salgadoguadalupe14@yahoo.com,7/28/2022,Guadalupe Salgado,254 W BARNETT ST,,VENTURA,CA,93001-1614,Shipped,AMZN_US(TBA191037690704),$12.99 ,$1.01 ,$14.00 ,FALSE,,FALSE,Guadalupe,USD,
7/28/2022,113-6152711-0397020,"HEELE Dog Muzzle, Mesh Cover Breathable Basket Muzzles for Small Medium Large Dogs, Soft Adjustable Strap Cage Muzzle, Prevent Biting, Licking and Chewing Black Small",ANIMAL_MUZZLE,B0B4P16XVD,10000000,Amazon.com,,new,Heele,,$29.99 ,$13.59 ,1,Discover7733,,,salgadoguadalupe14@yahoo.com,7/28/2022,Guadalupe Salgado,254 W BARNETT ST,,VENTURA,CA,93001-1614,Shipped,AMZN_US(TBA191259408004),$13.59 ,$0.95 ,$14.54 ,FALSE,,FALSE,Guadalupe,USD,
7/28/2022,113-6152711-0397020,"Dog Muzzle, Printed Dog Muzzles for Small Medium Large Dogs, Breathable Soft Muzzle for Dogs with Adjustable Velcro to Prevent Biting and Chewing, Allows Panting and Drinking",ANIMAL_MUZZLE,B0B1D7YLXJ,10131508,Amazon.com,,new,Maxlyn,,$15.99 ,$10.99 ,1,Discover7733,,,salgadoguadalupe14@yahoo.com,7/28/2022,Guadalupe Salgado,254 W BARNETT ST,,VENTURA,CA,93001-1614,Shipped,AMZN_US(TBA191037690704),$10.99 ,$0.81 ,$11.80 ,FALSE,,FALSE,Guadalupe,USD,`)

	//this gets info for each item from web request
	//TODO: add parallelization
	getoriginalprice(&orderhist)

	//let's now categorize each listing
	result := categorizeItems(&orderhist)
	return result
}

//populates our array with info from csv
func parseCSV(str string) []OrderInfo {
	s := strings.NewReader(str)

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

		/* deprecated, we don't actually need to do this!!!
		//let's do a little bit of editing on the name
		//if there is a comma in the name, it indicates quantities of the package. We want to just remove this part of the name
		//by taking a substring of the name until we get to the comma
		if x := strings.Index(s0, ","); x > -1 {
			s0 = s0[:x]
		}*/

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

//populates currente price and price drop in orderhist array
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

		//we first check if the item is unavailable, because if it is unavailable, we will get some NOT okay prices.
		//this tag should only appear on unavailable items
		if priceindex := strings.Index(strbody, `<span class="a-color-price a-text-bold">Currently unavailable.</span>`); priceindex != -1 {
			//unavailable, so let's just skip it
			//fmt.Println("Sorry, but, ", (*orderhist)[i].Name, " is currently unavailable.")
			continue
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

		//now lets actually modify our array
		(*orderhist)[i].CurrentPrice = price
		(*orderhist)[i].PriceDrop = math.Round(((*orderhist)[i].OriginalPrice-price)*100) / 100

		//done!
		resp.Body.Close()
		//fmt.Println("Price obtained from URL: ", getUrl((*orderhist)[i].Name, (*orderhist)[i].Asin), " is: ", price)
	}
}

func getUrl(name, asin string) string {
	//this structure is gotten from analyzing enough URLs, the name part can be literally anything, but this is what I chose
	itemurl := "https://www.amazon.com/" + name[:15] + "/dp/" + asin + "/"
	return itemurl
}

func categorizeItems(orderhist *[]OrderInfo) (ItemCategories, error) {
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

	return cat, nil
}

func main() {
		lambda.Start(New)
}
