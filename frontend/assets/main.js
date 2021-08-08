function setActivePage(page) {
  var active = document.querySelector('.page.active');
  if (active) {
    active.classList.remove('active');
  }
  document.querySelector('.page.' + page).classList.add('active');

  switch (page) {
    case 'lifts':
      // For lifts, we need to load the current lift.
      loadNextLift();
      break;
  }
}

function loadNextLift() {
  sendRequest('GET', '/api/nextLift', {}, function(rawResp, status) {
    if (status !== 200) {
      console.log('Error getting lifts', rawResp);
      return;
    }

    var resp = JSON.parse(rawResp);
    var liftHeader = document.getElementById('lift-header');
    liftHeader.textContent = resp.WeekName + ' - ' + resp.DayName

    var body = document.getElementById('lift-body');
    var listEl = document.createElement('ul');

    for (var i = 0; i < resp.Workout.length; i++) {
      var mvmt = resp.Workout[i];
      var mvmtEl = document.createElement('li');
      var mvmtList = document.createElement('ul');
      mvmtEl.textContent = mvmt.Exercise + ' - ' + mvmt.SetType;

      for (var j = 0; j < mvmt.Sets.length; j++) {
        var set = mvmt.Sets[j];
        var setEl = document.createElement('li');
        var setStr = set.RepTarget;
        if (set.ToFailure) {
          setStr += '+';
        }
        setStr += ' @ ' + set.TrainingMaxPercentage + '%';
        setEl.textContent = setStr;

        mvmtList.appendChild(setEl);
      }
      mvmtEl.appendChild(mvmtList)
      listEl.appendChild(mvmtEl);
    }

    body.appendChild(listEl);
  });
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
