import { Terminal, IEvent, IDisposable } from "xterm";

import base64 from './base64';
import { CryptoUtils } from './crypto';

interface IRectSize {
    width: number;
    height: number;
}

class TTYReceiver {
    private xterminal: Terminal;
    private containerElement: HTMLElement;
    private encryptionKey: Uint8Array | null;

    constructor(wsAddress: string, container: HTMLDivElement) {
        // Extract encryption key from URL hash
        this.encryptionKey = CryptoUtils.extractKeyFromHash();
        
        if (this.encryptionKey) {
            console.log("🔒 End-to-end encryption enabled");
        } else if (window.location.hash.startsWith('#key=')) {
            console.log("🔓 Invalid encryption key in URL");
        } else {
            console.log("🔓 No encryption key found - unencrypted session");
        }
        
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

        // Display encryption status in terminal
        setTimeout(() => {
            if (this.encryptionKey) {
                this.xterminal.write('\r\n🔒 End-to-end encryption: \x1b[32mENABLED\x1b[0m\r\n');
            } else if (window.location.hash.startsWith('#key=')) {
                this.xterminal.write('\r\n🔓 Encryption: \x1b[31mINVALID KEY\x1b[0m - will show encrypted data\r\n');
            }
            this.xterminal.write('\r\n');
        }, 100);

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

        connection.onmessage = async (ev: MessageEvent) => {
            let message = JSON.parse(ev.data)

            if (message.Type === "Encrypted") {
                await this.handleEncryptedMessage(message);
            } else {
                // Handle unencrypted messages (backward compatibility)
                this.handleUnencryptedMessage(message);
            }
        }

        this.xterminal.onData((data:string) => {
            this.sendInputData(connection, data);
        });

    }

    private async handleEncryptedMessage(message: any): Promise<void> {
        try {
            const msgData = base64.decode(message.Data);
            const encryptedMsg = JSON.parse(msgData);

            if (this.encryptionKey) {
                // Decrypt the message
                const encryptedData = base64.base64ToUint8Array(encryptedMsg.EncryptedData);
                const nonce = base64.base64ToUint8Array(encryptedMsg.Nonce);
                
                const decryptedData = await CryptoUtils.decryptData(encryptedData, nonce, this.encryptionKey);
                const decryptedText = new TextDecoder().decode(decryptedData);
                const decryptedMessage = JSON.parse(decryptedText);
                
                // Handle the decrypted message
                this.handleUnencryptedMessage(decryptedMessage);
            } else {
                // No encryption key - show encrypted data as-is
                const encryptedText = `\x1b[31m🔒 [ENCRYPTED]\x1b[0m `;
                this.xterminal.write(encryptedText);
            }
        } catch (error) {
            console.error('Failed to handle encrypted message:', error);
            this.xterminal.write('\r\n🔒 [ENCRYPTION ERROR]\r\n');
        }
    }

    private handleUnencryptedMessage(message: any): void {
        const msgData = base64.decode(message.Data);

        if (message.Type === "Write") {
            const writeMsg = JSON.parse(msgData);
            this.xterminal.write(base64.base64ToArrayBuffer(writeMsg.Data));
        }

        if (message.Type === "WinSize") {
            const winSizeMsg = JSON.parse(msgData);

            const containerPixSize = this.getElementPixelsSize(this.containerElement);
            const newFontSize = this.guessNewFontSize(winSizeMsg.Cols, winSizeMsg.Rows, containerPixSize.width, containerPixSize.height);
            this.xterminal.options.fontSize = newFontSize;

            // Now set the new size.
            this.xterminal.resize(winSizeMsg.Cols, winSizeMsg.Rows);
        }
    }

    private async sendInputData(connection: WebSocket, data: string): Promise<void> {
        try {
            if (this.encryptionKey) {
                // Encrypt the input data
                const writeMsg = { Size: data.length, Data: base64.encode(data) };
                const writeWrapper = { Type: "Write", Data: base64.encode(JSON.stringify(writeMsg)) };
                
                const plainData = new TextEncoder().encode(JSON.stringify(writeWrapper));
                const encrypted = await CryptoUtils.encryptData(plainData, this.encryptionKey);
                
                const encryptedMsg = {
                    Type: "Encrypted",
                    Data: base64.encode(JSON.stringify({
                        EncryptedData: base64.uint8ArrayToBase64(encrypted.encryptedData),
                        Nonce: base64.uint8ArrayToBase64(encrypted.nonce)
                    }))
                };
                
                connection.send(JSON.stringify(encryptedMsg));
            } else {
                // Send unencrypted (original behavior)
                const writeMessage = {
                    Type: "Write",
                    Data: base64.encode(JSON.stringify({ Size: data.length, Data: base64.encode(data)})),
                };
                connection.send(JSON.stringify(writeMessage));
            }
        } catch (error) {
            console.error('Failed to send input data:', error);
        }
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
