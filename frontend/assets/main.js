function setActivePage(page) {
  var active = document.querySelector('.page.active');
  if (active) {
    active.classList.remove('active');
  }
  document.querySelector('.page.' + page).classList.add('active');
}

function sendRequest(method, url, jsonData, callback) {
    var request = new XMLHttpRequest();

    request.onload = function() {
      callback(this.response, this.status);
    };

    request.onerror = function() {
      console.log('got a connection error');
    };

    request.open(method, url, true);
    request.setRequestHeader('Content-Type', 'application/json');
    request.send(JSON.stringify(jsonData));
}

function initialize() {
  sendRequest('GET', '/api/user', {}, function(rawResp, status) {
    if (status !== 200) {
      setActivePage('login');
      return;
    }

    var resp = JSON.parse(rawResp);
    if (resp.TrainingMaxes === null) {
      setActivePage('training-maxes');
    } else {
      setActivePage('lifts');
    }
  });
}

document.addEventListener('DOMContentLoaded', function(event) {
  // When the page loads, see if the user is already logged in and decide where to put them..
  initialize();

  document.getElementById('password-button').addEventListener('click', function(event) {
    var req = {
      password: document.getElementById('password-input').value,
    };
    sendRequest('POST', '/api/login', req, function(_resp, _status) {
      // Send the user to wherever they're supposed to be.
      initialize();
    })
  });

  document.getElementById('tm-button').addEventListener('click', function(event) {
    var req = {
      overhead_press: document.getElementById('press-input').value,
      squat: document.getElementById('squat-input').value,
      bench_press: document.getElementById('bench-input').value,
      deadlift: document.getElementById('deadlift-input').value,
    };

    sendRequest('POST', '/api/setTrainingMaxes', req, function(_resp, _status) {
      setActivePage('lifts');
    })
  })
});
