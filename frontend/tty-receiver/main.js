import 'xterm/dist/xterm.css';
import { Terminal } from 'xterm';
import pbkdf2 from 'pbkdf2';

import React from 'react';
import ReactDOM from 'react-dom';
import App from './app';
import base64 from './base64'


ReactDOM.render(
    <App />,
    document.querySelector('#settings')
);

var term = new Terminal({
    cursorBlink: true,
    macOptionIsMeta: true,
});

var derivedKey = pbkdf2.pbkdf2Sync('password', 'salt', 4096, 32, 'sha256');
console.log(derivedKey);

let wsAddress = "";
if (window.location.protocol === "https:") {
   wsAddress = 'wss://';
} else {
    wsAddress = "ws://";
}

wsAddress += window.location.host + window.ttyInitialData.wsPath;
let connection = new WebSocket(wsAddress);



term.open(document.getElementById('terminal'), true);

term.write("$");

connection.onclose = function(evt) {
    console.log("Got the WS closed: ", evt);
    window.location.reload();
}

connection.onmessage = function(evt) {
    let message = JSON.parse(evt.data)

    let msgData = base64.decode(message.Data)

    if (message.Type === "Write") {
        let writeMsg = JSON.parse(msgData)
        term.write(base64.decode(writeMsg.Data))
    }

    if (message.Type == "WinSize") {
        let winSizeMsg = JSON.parse(msgData)
        term.resize(winSizeMsg.Cols, winSizeMsg.Rows)
    }
}

term.on('data', function (data) {
    //console.log('TERM->WS:', data);
    let writeMessage = {
        Type: "Write",
        Data: base64.encode(JSON.stringify({ Size: data.length, Data: base64.encode(data)})),
    }
    let dataToSend = JSON.stringify(writeMessage)
    //console.log("Sending : ", dataToSend)
    connection.send(dataToSend);

})