const endpoint = "https://www.wiztim.dev/refund";



function uploadCSVFile() {
    const file = document.getElementById("inpFile").files[0];

    var fr = new FileReader();
    var info;
    var output;
    fr.readAsText(file);
    fr.onload = () => {
        info = fr.result;
        csvContents = info.replace(/[\r\n]/gm, '\\n');
        csvContents = csvContents.replace(/[\r"]/gm, '\\"');
        csvContents = "\"" + csvContents + "\"";
        fetch(endpoint, {
            method: "POST",
            body: csvContents
        })
            .then((response) => response.json())
            .then((data) => {
                document.getElementById("upload").style.display = "none";
                document.getElementById("csv").style.display = "block";
                console.log(data);

                const reduced = data.priceReduced;
                const unchanged = data.priceUnchanged;
                const increased = data.princeIncreased;
                const unavailable = data.unavailable;
                console.log(reduced);
                console.log(unchanged);
                console.log(increased);
                console.log(unavailable);
                if (reduced != null) {
                    for (let i = 0; i < reduced.length; i++) {
                        document.getElementById("reduced").innerHTML += reduced[i].name + "<br>";
                    }
                } else {
                    document.getElementById("reduced").innerHTML += "Nothing"
                }
                if (unchanged != null) {
                    for (let i = 0; i < unchanged.length; i++) {
                        document.getElementById("unchanged").innerHTML += unchanged[i].name + "<br>";
                    }
                } else {
                    document.getElementById("reduced").innerHTML += "Nothing"
                }
                if (increased != null) {
                    for (let i = 0; i < increased.length; i++) {
                        document.getElementById("increased").innerHTML += increased[i].name + "<br>";
                    }
                } else {
                    document.getElementById("reduced").innerHTML += "Nothing"
                }
                if (unavailable != null) {
                    for (let i = 0; i < unavailable.length; i++) {
                        document.getElementById("unavailable").innerHTML += unavailable[i].name + "<br>";
                    }
                } else {
                    document.getElementById("reduced").innerHTML += "Nothing"
                }
            });
    }
}

function fileValidation() {
    var filePath = document.getElementById('inpFile').value;

    var allowedExtension = /(\.csv)$/i;
    if (!allowedExtension.exec(filePath)) {
        alert('Invalid File Type');
        filePath = '';
        return false;
    }

    const file = document.getElementById("inpFile").files[0];

    // const csvOpener = new RegExp("Order Date,Order ID,Title,Category,ASIN/ISBN,UNSPSC Code,Website,Release Date,Condition,Seller,Seller Credentials,List Price Per Unit,Purchase Price Per Unit,Quantity,Payment Instrument Type,Purchase Order Number,PO Line Number,Ordering Customer Email,Shipment Date,Shipping Address Name,Shipping Address Street 1,Shipping Address Street 2,Shipping Address City,Shipping Address State,Shipping Address Zip,Order Status,Carrier Name & Tracking Number,Item Subtotal,Item Subtotal Tax,Item Total,Tax Exemption Applied,Tax Exemption Type,Exemption Opt-Out,Buyer Name,Currency,Group Name*");

    // var fr = new FileReader();
    // fr.readAsText(file);
    // var info;
    // fr.onload = function(event) {
    //     info = fr.result;
    // }
    // if (!csvOpener.exec(info)){
    //     alert('Invalid CSV Format - Please use the Items Report Type');
    //     filePath = '';
    //     return false;
    // }
    return true;

}