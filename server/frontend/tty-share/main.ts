import 'xterm/css/xterm.css';
import './main.css';

import { Terminal } from 'xterm';

import { TTYReceiver } from './tty-receiver';

const term = new Terminal({
    cursorBlink: true,
    macOptionIsMeta: true,
});

let wsAddress = "";
if (window.location.protocol === "https:") {
   wsAddress = 'wss://';
} else {
    wsAddress = "ws://";
}

let ttyWindow = window as any;
wsAddress += ttyWindow.location.host + ttyWindow.ttyInitialData.wsPath;


const ttyReceiver = new TTYReceiver(wsAddress, document.getElementById('terminal') as HTMLDivElement);
