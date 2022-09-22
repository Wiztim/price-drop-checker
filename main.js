const endpoint = "https://www.wiztim.dev/dummy";

function uploadCSVData() {
    const file = document.getElementById("inpFile").files[0];

    var fr = new FileReader();
    fr.readAsText(file);
    fr.onload = function(event) {
        const info = fr.result;
    }

    fetch(endpoint, {
        method: "post",
        body: info
    }).catch(console.error);


}

function fileValidation(){
    var filePath = document.getElementById('inpFile').value;
    
    var allowedExtension = /(\.csv)$/i;
    if (!allowedExtension.exec(filePath)){
        alert('Invalid File Type');
        filePath = '';
        return false;
    }

    const file = document.getElementById("inpFile").files[0];
    
    const csvOpener = new RegExp("Order Date,Order ID,Title,Category,ASIN/ISBN,UNSPSC Code,Website,Release Date,Condition,Seller,Seller Credentials,List Price Per Unit,Purchase Price Per Unit,Quantity,Payment Instrument Type,Purchase Order Number,PO Line Number,Ordering Customer Email,Shipment Date,Shipping Address Name,Shipping Address Street 1,Shipping Address Street 2,Shipping Address City,Shipping Address State,Shipping Address Zip,Order Status,Carrier Name & Tracking Number,Item Subtotal,Item Subtotal Tax,Item Total,Tax Exemption Applied,Tax Exemption Type,Exemption Opt-Out,Buyer Name,Currency,Group Name\n");

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
    return true;

}