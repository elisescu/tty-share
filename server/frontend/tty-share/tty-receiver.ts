import { Terminal, IEvent, IDisposable } from "xterm";

import base64 from './base64';

interface IRectSize {
    width: number;
    height: number;
}

class TTYReceiver {
    private xterminal: Terminal;
    private containerElement: HTMLElement;

    constructor(wsAddress: string, container: HTMLDivElement) {
        console.log("Opening WS connection to ", wsAddress)
        const connection = new WebSocket(wsAddress);

        // TODO: expose some of these options in the UI
        this.xterminal = new Terminal({
            cursorBlink: true,
            macOptionIsMeta: true,
            scrollback: 1000,
            fontSize: 12,
            letterSpacing: 0,
            fontFamily: 'SauceCodePro MonoWindows, courier-new, monospace',
        });

        this.containerElement = container;
        this.xterminal.open(container);

        connection.onclose =  (evt: CloseEvent) => {

           this.xterminal.blur();
           this.xterminal.options.cursorBlink = false
           this.xterminal.clear();

           setTimeout(() => {
            this.xterminal.write('Session closed');
           }, 1000)
        }

        this.xterminal.focus();

        const containerPixSize = this.getElementPixelsSize(container);
        const newFontSize = this.guessNewFontSize(this.xterminal.cols, this.xterminal.rows, containerPixSize.width, containerPixSize.height);
        this.xterminal.options.fontSize = newFontSize
        this.xterminal.options.fontFamily= 'SauceCodePro MonoWindows, courier-new, monospace'

        connection.onmessage = (ev: MessageEvent) => {
            let message = JSON.parse(ev.data)
            let msgData = base64.decode(message.Data)

            if (message.Type === "Write") {
                let writeMsg = JSON.parse(msgData)
                this.xterminal.write(base64.base64ToArrayBuffer(writeMsg.Data));
            }

            if (message.Type == "WinSize") {
                let winSizeMsg = JSON.parse(msgData)

                const containerPixSize = this.getElementPixelsSize(container);
                const newFontSize = this.guessNewFontSize(winSizeMsg.Cols, winSizeMsg.Rows, containerPixSize.width, containerPixSize.height);
                this.xterminal.options.fontSize = newFontSize

                // Now set the new size.
                this.xterminal.resize(winSizeMsg.Cols, winSizeMsg.Rows)
            }
        }

        this.xterminal.onData(function (data:string) {
            let writeMessage = {
                Type: "Write",
                Data: base64.encode(JSON.stringify({ Size: data.length, Data: base64.encode(data)})),
            }
            let dataToSend = JSON.stringify(writeMessage)
            connection.send(dataToSend);
        });

    }

    // Get the pixels size of the element, after all CSS was applied. This will be used in an ugly
    // hack to guess what fontSize to set on the xterm object. Horrible hack, but I feel less bad
    // about it seeing that VSV does it too:
    // https://github.com/microsoft/vscode/blob/d14ee7613fcead91c5c3c2bddbf288c0462be876/src/vs/workbench/parts/terminal/electron-browser/terminalInstance.ts#L363
    private getElementPixelsSize(element: HTMLElement): IRectSize {
        const defView = this.containerElement.ownerDocument.defaultView;
        let width = parseInt(defView.getComputedStyle(element).getPropertyValue('width').replace('px', ''), 10);
        let height = parseInt(defView.getComputedStyle(element).getPropertyValue('height').replace('px', ''), 10);

        return {
            width,
            height,
        }
    }

    // Tries to guess the new font size, for the new terminal size, so that the rendered terminal
    // will have the newWidth and newHeight dimensions
    private guessNewFontSize(newCols: number, newRows: number, targetWidth: number, targetHeight: number): number {
        const cols = this.xterminal.cols;
        const rows = this.xterminal.rows;
        const fontSize = this.xterminal.options.fontSize
        const xtermPixelsSize = this.getElementPixelsSize(this.containerElement.querySelector(".xterm-screen"));

        const newHFontSizeMultiplier =  (cols / newCols) * (targetWidth / xtermPixelsSize.width);
        const newVFontSizeMultiplier = (rows / newRows) * (targetHeight / xtermPixelsSize.height);

        let newFontSize;

        if (newHFontSizeMultiplier > newVFontSizeMultiplier) {
            newFontSize = Math.floor(fontSize * newVFontSizeMultiplier);
        } else {
            newFontSize = Math.floor(fontSize * newHFontSizeMultiplier);
        }
        return newFontSize;
    }
}

export {
    TTYReceiver
}
