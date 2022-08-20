// import axios from 'axios';
const axios = require('axios');
const express = require('express');
const multer = require('multer');

const http = require('http');
const fs = require('fs');
const cors = require('cors');
const port = 3000;

const server = http.createServer(function(req, res){
    res.writeHead(200, {})
    fs.readFile('homepage.html', function(error, data){
        if (error) {
            res.writeHead(404)
            res.write('Error: File Not Found')
        } else {
            res.write(data)
        }
        res.end()
    })
})

server.listen(port, function(error){
    if (error){
        console.log('Something went wrong', error)
    } else {
        console.log('Server listening at port ' + port)
    }
})

const app = express();
app.use(cors());
const upload = multer();

const SERVER_URL = "https://www.wiztim.dev/dummy";

let formData = new FormData();

// function getData(){
//     return fetch
// }