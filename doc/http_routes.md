Routes
======

These are the routes the server will listen to:

* `/` - the main page which probably will be a redirect
* `/ws/<session id>` - will serve the websockets session
* `/s/<session id>` - will serve the tty-receiver webpage, which will make some further requests for
  the resources
* `/static/` - serving the static resources: 404 page, js and css files
