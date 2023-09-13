const endpoint = "http://localhost:8080";



function uploadCSVFile() {
    // get the uploaded CSV file
    const file = document.getElementById("inpFile").files[0];
    if (file == null) {
        alert("Please upload a file.");
        return;
    }

    // when the file uploads, use regex to fix the files new line characters and quotation marks
    let fr = new FileReader();
    fr.readAsText(file);
    fr.onload = () => {
        let csvContents = fr.result;
        // this if for lambda
        //csvContents = csvContents.replace(/[\r\n]/gm, '\\n');
        //csvContents = csvContents.replace(/[\r"]/gm, '\\"');
        //csvContents = "\"" + csvContents + "\"";

        // create the API call, and display it when it is finished
        fetch(endpoint, {
            method: "POST",
            body: csvContents
        })
            .then((response) => response.json())
            .then((data) => {
                // hide the file upload html, and reveal the data html
                document.getElementById("upload").style.display = "none";
                document.getElementById("csv").style.display = "block";

                // for each category of products, display them in a list
                const reduced = data.priceReduced;
                const unchanged = data.priceUnchanged;
                const increased = data.priceIncreased;
                const unavailable = data.unavailable;

                if (reduced != null) {
                    var reducedHTML = document.getElementById("reduced");
                    for (let i = 0; i < reduced.length; i++) {
                        let product = reduced[i];
                        reducedHTML.innerHTML += "<a href=\"https://www.amazon.com/dp/" + product.asin + "\"/>" + product.name + "</a><br>";
                        reducedHTML.innerHTML += "Date Ordered: " + product.dateOrdered + "<br>";
                        reducedHTML.innerHTML += "Original Price: $" + product.originalPrice + "<br>";
                        reducedHTML.innerHTML += "Reduced Price: $" + product.currentPrice + "<hr>";
                    }
                } else {
                    reducedHTML.innerHTML += "Nothing"
                }
                if (unchanged != null) {
                    var unchangedHTML = document.getElementById("unchanged");
                    for (let i = 0; i < unchanged.length; i++) {
                        let product = unchanged[i];
                        unchangedHTML.innerHTML += "<a href=\"https://www.amazon.com/dp/" + product.asin + "\"/>" + product.name + "</a><br>";
                        unchangedHTML.innerHTML += "Date Ordered: " + product.dateOrdered + "<br>";
                        unchangedHTML.innerHTML += "Original Price: $" + product.originalPrice + "<hr>";
                    }
                } else {
                    unchangedHTML.innerHTML += "Nothing"
                }
                if (increased != null) {
                    var increasedHTML = document.getElementById("increased");
                    for (let i = 0; i < increased.length; i++) {
                        let product = increased[i];
                        increasedHTML.innerHTML += "<a href=\"https://www.amazon.com/dp/" + product.asin + "\"/>" + product.name + "</a><br>";
                        increasedHTML.innerHTML += "Date Ordered: " + product.dateOrdered + "<br>";
                        increasedHTML.innerHTML += "Original Price: $" + product.originalPrice + "<br>";
                        increasedHTML.innerHTML += "Increased Price: $" + product.currentPrice + "<hr>";
                    }
                } else {
                    increasedHTML.innerHTML += "Nothing"
                }
                if (unavailable != null) {
                    var unavailableHTML = document.getElementById("unavailable");
                    for (let i = 0; i < unavailable.length; i++) {
                        let product = unavailable[i];
                        unavailableHTML.innerHTML += "<a href=\"https://www.amazon.com/dp/" + product.asin + "\"/>" + product.name + "</a><br>";
                        unavailableHTML.innerHTML += "Date Ordered: " + product.dateOrdered + "<br>";
                        unavailableHTML.innerHTML += "Original Price: $" + product.originalPrice + "<hr>";
                    }
                } else {
                    unavailableHTML.innerHTML += "Nothing"
                }
            });
    }
}

function fileValidation() {
    // check the chosen file's extension
    let filePath = document.getElementById('inpFile').value;
    let allowedExtension = /(\.csv)$/i;

    if (!allowedExtension.exec(filePath)) {
        alert('Invalid File Type');
        filePath = '';
        return false;
    }


    //attempted to check the first line of the CSV, but it seems that the CSV is inconsistant
    /*
    const file = document.getElementById("inpFile").files[0];
    const csvOpener = new RegExp("Order Date,Order ID,Title,Category,ASIN/ISBN,UNSPSC Code,Website,Release Date,Condition,Seller,Seller Credentials,List Price Per Unit,Purchase Price Per Unit,Quantity,Payment Instrument Type,Purchase Order Number,PO Line Number,Ordering Customer Email,Shipment Date,Shipping Address Name,Shipping Address Street 1,Shipping Address Street 2,Shipping Address City,Shipping Address State,Shipping Address Zip,Order Status,Carrier Name & Tracking Number,Item Subtotal,Item Subtotal Tax,Item Total,Tax Exemption Applied,Tax Exemption Type,Exemption Opt-Out,Buyer Name,Currency,Group Name*");

    var fr = new FileReader();
    fr.readAsText(file);
    var info;
    fr.onload = function(event) {
        info = fr.result;
    }
    if (!csvOpener.exec(info)){
        alert('Invalid CSV Format - Please use the Items Report Type');
        filePath = '';
        return false;
    }
    */
    return true;

}