(function() {
    if (Cookies.get("access_token") === undefined) {
        console.log('Setting as login');
        $('#login').text("Log In");
        document.getElementById('login').addEventListener('click', function () {
            window.location.replace("/login");
        })
    } else {
        console.log('Setting as logout');
        $('#login').text("Log Out");
        document.getElementById('login').addEventListener('click', function () {
            Cookies.set("access_token", "", { expires: -1 });
            window.location.replace("/");
        })
    }
})();
