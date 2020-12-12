# TO DO LIST

This list contains things that should be done next with Ego. This is a dynamic list.

1. validate(user, pass) function in server mode that validates a username and password against 
   the active users database. Returns true if names are valid.

2. rest.new([url]) function that creates a new rest call. If the url is not given, it
   must be supplied in a subsequent call. Returns an opaque package with the following
   functions available:
   * r.url(s) sets the url
   * r.header("header", "value") sets a header value
   * r.body(string) sets the request body
   * r.mode(string) sets the mode "GET", "POST", etc.
   * r.request() sends the request.
   * r.header("header") retrieves a header value
   * r.response() retrieves the response body
   * r.status() returns the status of the call
   * r.close() discards the object
   The current active token is automatically attached as a header to the response
   when the request is made.

3. Profile setting for "login-server" which is the URL of the Ego server to call 
   for login operations. Stores a "login-token" in the profile. 

3. Add login verb. Accepts user and password options; if not given will prompt for them.
   Accepts login-server option to override/set the login-server profile item. Successful
   login stores a token back in the profile for future use.

4. Create service on the login server that validates a token. When presented with a token
   as payload, will return an error or the user data string for the token to the caller.
   This would be used by any other service that uses the Ego server for login/authentication.
   