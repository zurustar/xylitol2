// Admin interface JavaScript functionality

document.addEventListener('DOMContentLoaded', function() {
    // Form validation for user creation and editing
    const userForm = document.getElementById('userForm');
    const editUserForm = document.getElementById('editUserForm');
    const searchInput = document.getElementById('userSearch');
    
    if (userForm) {
        userForm.addEventListener('submit', validateUserForm);
    }
    
    if (editUserForm) {
        editUserForm.addEventListener('submit', validateEditUserForm);
        // Handle PUT method for form submission
        editUserForm.addEventListener('submit', function(e) {
            e.preventDefault();
            submitEditForm(this);
        });
    }
    
    // Initialize search functionality
    if (searchInput) {
        searchInput.addEventListener('input', debounce(filterUsers, 300));
    }
    
    // Initialize user status toggle functionality
    initializeStatusToggles();
});

// Validate user creation form
function validateUserForm(event) {
    event.preventDefault();
    
    const username = document.getElementById('username').value.trim();
    const realm = document.getElementById('realm').value.trim();
    const password = document.getElementById('password').value;
    const confirmPassword = document.getElementById('confirm-password').value;
    
    let isValid = true;
    
    // Clear previous errors
    clearErrors();
    
    // Validate username
    if (!username) {
        showError('username-error', 'Username is required');
        isValid = false;
    } else if (username.length < 3) {
        showError('username-error', 'Username must be at least 3 characters');
        isValid = false;
    }
    
    // Validate realm
    if (!realm) {
        showError('realm-error', 'Realm is required');
        isValid = false;
    }
    
    // Validate password
    if (!password) {
        showError('password-error', 'Password is required');
        isValid = false;
    } else if (password.length < 6) {
        showError('password-error', 'Password must be at least 6 characters');
        isValid = false;
    }
    
    // Validate password confirmation
    if (password !== confirmPassword) {
        showError('confirm-password-error', 'Passwords do not match');
        isValid = false;
    }
    
    if (isValid) {
        event.target.submit();
    }
}

// Validate user edit form
function validateEditUserForm(event) {
    const password = document.getElementById('password').value;
    const confirmPassword = document.getElementById('confirm-password').value;
    
    let isValid = true;
    
    // Clear previous errors
    clearErrors();
    
    // Only validate password if it's being changed
    if (password || confirmPassword) {
        if (password.length < 6) {
            showError('password-error', 'Password must be at least 6 characters');
            isValid = false;
        }
        
        if (password !== confirmPassword) {
            showError('confirm-password-error', 'Passwords do not match');
            isValid = false;
        }
    }
    
    return isValid;
}

// Submit edit form with PUT method
function submitEditForm(form) {
    if (!validateEditUserForm({ target: form })) {
        return;
    }
    
    const formData = new FormData(form);
    const actionUrl = form.action;
    
    fetch(actionUrl, {
        method: 'PUT',
        body: formData
    })
    .then(response => {
        if (response.ok) {
            window.location.href = '/admin';
        } else {
            return response.text().then(text => {
                throw new Error(text);
            });
        }
    })
    .catch(error => {
        alert('Error updating user: ' + error.message);
    });
}

// Delete user function
function deleteUser(username, realm, userId) {
    if (!confirm(`Are you sure you want to delete user "${username}@${realm}"?`)) {
        return;
    }
    
    const formData = new FormData();
    formData.append('username', username);
    formData.append('realm', realm);
    
    fetch(`/admin/users/${userId}`, {
        method: 'DELETE',
        body: formData
    })
    .then(response => {
        if (response.ok) {
            location.reload();
        } else {
            return response.text().then(text => {
                throw new Error(text);
            });
        }
    })
    .catch(error => {
        alert('Error deleting user: ' + error.message);
    });
}

// Helper functions
function showError(elementId, message) {
    const errorElement = document.getElementById(elementId);
    if (errorElement) {
        errorElement.textContent = message;
        errorElement.style.display = 'block';
    }
}

function clearErrors() {
    const errorElements = document.querySelectorAll('.error-message');
    errorElements.forEach(element => {
        element.textContent = '';
        element.style.display = 'none';
    });
}

// API helper functions
function apiCall(url, method, data) {
    const options = {
        method: method,
        headers: {
            'Content-Type': 'application/json',
        }
    };
    
    if (data) {
        options.body = JSON.stringify(data);
    }
    
    return fetch(url, options)
        .then(response => {
            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }
            return response.json();
        });
}

// Refresh user list (for future AJAX functionality)
function refreshUserList() {
    apiCall('/admin/users', 'GET')
        .then(users => {
            // Update user table dynamically
            updateUserTable(users);
        })
        .catch(error => {
            console.error('Error refreshing user list:', error);
        });
}

function updateUserTable(users) {
    // This function would update the user table without page reload
    // Implementation depends on specific requirements
    console.log('Updating user table with:', users);
}
// Se
arch and filter functionality
function filterUsers() {
    const searchTerm = document.getElementById('userSearch').value.toLowerCase();
    const userRows = document.querySelectorAll('.user-row');
    
    userRows.forEach(row => {
        const username = row.querySelector('.username').textContent.toLowerCase();
        const realm = row.querySelector('.realm').textContent.toLowerCase();
        const searchText = username + ' ' + realm;
        
        if (searchText.includes(searchTerm)) {
            row.style.display = '';
        } else {
            row.style.display = 'none';
        }
    });
    
    // Update no results message
    updateNoResultsMessage();
}

function updateNoResultsMessage() {
    const userRows = document.querySelectorAll('.user-row');
    const visibleRows = Array.from(userRows).filter(row => row.style.display !== 'none');
    const noResultsRow = document.getElementById('no-results-row');
    
    if (visibleRows.length === 0 && userRows.length > 0) {
        if (!noResultsRow) {
            const tbody = document.querySelector('.users-table tbody');
            const row = document.createElement('tr');
            row.id = 'no-results-row';
            row.innerHTML = '<td colspan="5" class="no-data">No users match your search criteria.</td>';
            tbody.appendChild(row);
        }
    } else if (noResultsRow) {
        noResultsRow.remove();
    }
}

// User status management
function initializeStatusToggles() {
    const statusButtons = document.querySelectorAll('.status-toggle');
    statusButtons.forEach(button => {
        button.addEventListener('click', toggleUserStatus);
    });
}

function toggleUserStatus(event) {
    const button = event.target;
    const userId = button.dataset.userId;
    const username = button.dataset.username;
    const realm = button.dataset.realm;
    const currentStatus = button.dataset.enabled === 'true';
    
    if (!confirm(`Are you sure you want to ${currentStatus ? 'disable' : 'enable'} user "${username}@${realm}"?`)) {
        return;
    }
    
    // This would require backend support for user status management
    // For now, just show a message
    alert('User status management requires backend implementation');
}

// Enhanced form validation with real-time feedback
function validateUserForm(event) {
    event.preventDefault();
    
    const username = document.getElementById('username').value.trim();
    const realm = document.getElementById('realm').value.trim();
    const password = document.getElementById('password').value;
    const confirmPassword = document.getElementById('confirm-password').value;
    
    let isValid = true;
    
    // Clear previous errors
    clearErrors();
    
    // Real-time validation
    isValid = validateUsername(username) && isValid;
    isValid = validateRealm(realm) && isValid;
    isValid = validatePassword(password) && isValid;
    isValid = validatePasswordConfirmation(password, confirmPassword) && isValid;
    
    if (isValid) {
        event.target.submit();
    }
}

function validateUsername(username) {
    if (!username) {
        showError('username-error', 'Username is required');
        return false;
    } else if (username.length < 3) {
        showError('username-error', 'Username must be at least 3 characters');
        return false;
    } else if (!/^[a-zA-Z0-9._-]+$/.test(username)) {
        showError('username-error', 'Username can only contain letters, numbers, dots, underscores, and hyphens');
        return false;
    }
    return true;
}

function validateRealm(realm) {
    if (!realm) {
        showError('realm-error', 'Realm is required');
        return false;
    } else if (!/^[a-zA-Z0-9.-]+$/.test(realm)) {
        showError('realm-error', 'Realm must be a valid domain name');
        return false;
    }
    return true;
}

function validatePassword(password) {
    if (!password) {
        showError('password-error', 'Password is required');
        return false;
    } else if (password.length < 6) {
        showError('password-error', 'Password must be at least 6 characters');
        return false;
    } else if (!/(?=.*[a-z])(?=.*[A-Z])(?=.*\d)/.test(password)) {
        showError('password-error', 'Password must contain at least one uppercase letter, one lowercase letter, and one number');
        return false;
    }
    return true;
}

function validatePasswordConfirmation(password, confirmPassword) {
    if (password !== confirmPassword) {
        showError('confirm-password-error', 'Passwords do not match');
        return false;
    }
    return true;
}

// Real-time validation for input fields
function setupRealTimeValidation() {
    const usernameInput = document.getElementById('username');
    const realmInput = document.getElementById('realm');
    const passwordInput = document.getElementById('password');
    const confirmPasswordInput = document.getElementById('confirm-password');
    
    if (usernameInput) {
        usernameInput.addEventListener('blur', () => validateUsername(usernameInput.value.trim()));
    }
    
    if (realmInput) {
        realmInput.addEventListener('blur', () => validateRealm(realmInput.value.trim()));
    }
    
    if (passwordInput) {
        passwordInput.addEventListener('blur', () => validatePassword(passwordInput.value));
    }
    
    if (confirmPasswordInput && passwordInput) {
        confirmPasswordInput.addEventListener('blur', () => 
            validatePasswordConfirmation(passwordInput.value, confirmPasswordInput.value));
    }
}

// Bulk operations
function selectAllUsers() {
    const checkboxes = document.querySelectorAll('.user-checkbox');
    const selectAllCheckbox = document.getElementById('select-all');
    
    checkboxes.forEach(checkbox => {
        checkbox.checked = selectAllCheckbox.checked;
    });
    
    updateBulkActions();
}

function updateBulkActions() {
    const checkedBoxes = document.querySelectorAll('.user-checkbox:checked');
    const bulkActions = document.getElementById('bulk-actions');
    
    if (checkedBoxes.length > 0) {
        bulkActions.style.display = 'block';
        document.getElementById('selected-count').textContent = checkedBoxes.length;
    } else {
        bulkActions.style.display = 'none';
    }
}

function bulkDeleteUsers() {
    const checkedBoxes = document.querySelectorAll('.user-checkbox:checked');
    const userCount = checkedBoxes.length;
    
    if (userCount === 0) {
        alert('Please select users to delete');
        return;
    }
    
    if (!confirm(`Are you sure you want to delete ${userCount} user(s)?`)) {
        return;
    }
    
    // Collect user information
    const usersToDelete = Array.from(checkedBoxes).map(checkbox => ({
        id: checkbox.dataset.userId,
        username: checkbox.dataset.username,
        realm: checkbox.dataset.realm
    }));
    
    // Delete users one by one (in a real implementation, this would be a batch operation)
    Promise.all(usersToDelete.map(user => 
        fetch(`/admin/users/${user.id}?username=${user.username}&realm=${user.realm}`, {
            method: 'DELETE'
        })
    )).then(responses => {
        const allSuccessful = responses.every(response => response.ok);
        if (allSuccessful) {
            location.reload();
        } else {
            alert('Some users could not be deleted. Please refresh and try again.');
        }
    }).catch(error => {
        alert('Error deleting users: ' + error.message);
    });
}

// Utility functions
function debounce(func, wait) {
    let timeout;
    return function executedFunction(...args) {
        const later = () => {
            clearTimeout(timeout);
            func(...args);
        };
        clearTimeout(timeout);
        timeout = setTimeout(later, wait);
    };
}

// Initialize real-time validation when DOM is loaded
document.addEventListener('DOMContentLoaded', setupRealTimeValidation);