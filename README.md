# Known Issue

- Amazon throttles calls to it's sites without using an API, resulting in many unavailable items. Unimplemented fix: use cookie header
- Currently self hosts the go code

# Price Drop Checker

- This is a program that checks an Amazon order history CSV for items that are within return window and their price.
- The backend was written in GO Lang and hosted on AWS API Gateway. The frontend was written in React JS and is hosted on AWS S3.
- Created by Timothy Orlov, Noah Terminello, and Aditya Sriram

# Usage

- Install the [Amazon Order History Reporter](https://chrome.google.com/webstore/detail/amazon-order-history-repo/mgkilgclilajckgnedgjgnfdokkgnibi) google extension
- Uncheck all boxes except
    - Show items not orders
    - Show ASIN in items view
- Recieve a list of items with their current price V.S. original price

# Frontend details

- Handles recieving the CSV
- Fixes the CSV for the backend to parse
- POST the CSV to the backend
- Displays a list of item names, the order date, and their prices from backend

# Backend details

- Recieves the CSV as plaintext
- Parses each item
- Webscrapes the item's product page for the current price
- Returns the following: product name, date ordered, ASIN, original price, current price, and price difference
