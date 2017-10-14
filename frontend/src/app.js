import React, { Component } from 'react';
import ReactDOM from 'react-dom';
import MuiThemePro from 'material-ui/styles/MuiThemeProvider';
import RaisedButton from 'material-ui/RaisedButton';
import TextField from 'material-ui/TextField';

class App extends Component {
    constructor(props) {
        super(props);
    }
    render() {
        return (<div> </div>);
        return (
            <MuiThemePro>
                <TextField
                    hintText="Password Field"
                    floatingLabelText="Password"
                    type="password"
                />
            </MuiThemePro>
        );
    }
}
export default App;