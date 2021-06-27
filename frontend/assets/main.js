function setActivePage(page) {
  document.querySelector('.page.active').classList.remove('active');
  document.querySelector('.page.' + page).classList.add('active');
}

function sendRequest(method, url, jsonData, successCallback) {
    var request = new XMLHttpRequest();

    request.onload = function() {
      if (this.status >= 200 && this.status < 400) {
        // Redirect to the main page.
        successCallback(this.response);
      } else {
        // We reached our target server, but it returned an error
        console.log('got an error' + this.status);
      }
    };

    request.onerror = function() {
      console.log('got a connection error');
    };

    request.open(method, url, true);
    request.setRequestHeader('Content-Type', 'application/json');
    request.send(JSON.stringify(jsonData));
}

document.addEventListener('DOMContentLoaded', function(event) {

  document.getElementById('password-button').addEventListener('click', function(event) {
    var req = {
      password: document.getElementById('password-input').value,
    };
    sendRequest('POST', '/api/login', req, function(_resp) {
      setActivePage('one-rep-maxes');
    })
  });

  document.getElementById('orm-button').addEventListener('click', function(event) {
    var req = {
      press: document.getElementById('press-input').value,
      squat: document.getElementById('squat-input').value,
      bench: document.getElementById('bench-input').value,
      deadlift: document.getElementById('deadlift-input').value,
    };

    sendRequest('POST', '/api/set-training-maxes', req, function(_resp) {
      setActivePage('lifts');
    })
  })
});
