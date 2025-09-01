// SIP Server Admin Interface JavaScript

// Utility functions
function showAlert(message, type = 'info') {
    const alertDiv = document.createElement('div');
    alertDiv.className = `alert alert-${type}`;
    alertDiv.textContent = message;
    
    const container = document.querySelector('.container');
    container.insertBefore(alertDiv, container.firstChild);
    
    // Auto-remove after 5 seconds
    setTimeout(() => {
        if (alertDiv.parentNode) {
            alertDiv.parentNode.removeChild(alertDiv);
        }
    }, 5000);
}

function confirmAction(message, callback) {
    if (confirm(message)) {
        callback();
    }
}

// User management functions
function deleteUser(userId) {
    confirmAction('Are you sure you want to delete this user? This action cannot be undone.', () => {
        fetch(`/admin/users/${userId}`, {
            method: 'DELETE'
        })
        .then(response => {
            if (response.ok) {
                showAlert('User deleted successfully', 'success');
                setTimeout(() => location.reload(), 1000);
            } else {
                showAlert('Failed to delete user', 'error');
            }
        })
        .catch(error => {
            console.error('Error:', error);
            showAlert('An error occurred while deleting the user', 'error');
        });
    });
}

// Hunt Group management functions
function deleteHuntGroup(groupId) {
    confirmAction('Are you sure you want to delete this hunt group? This action cannot be undone.', () => {
        fetch(`/admin/huntgroups/${groupId}`, {
            method: 'DELETE'
        })
        .then(response => {
            if (response.ok) {
                showAlert('Hunt group deleted successfully', 'success');
                setTimeout(() => location.reload(), 1000);
            } else {
                showAlert('Failed to delete hunt group', 'error');
            }
        })
        .catch(error => {
            console.error('Error:', error);
            showAlert('An error occurred while deleting the hunt group', 'error');
        });
    });
}

function viewStatistics(groupId) {
    fetch(`/admin/huntgroups/${groupId}/statistics`)
        .then(response => response.json())
        .then(stats => {
            let message = 'Hunt Group Statistics:\n\n';
            message += `Total Calls: ${stats.total_calls}\n`;
            message += `Answered Calls: ${stats.answered_calls}\n`;
            message += `Missed Calls: ${stats.missed_calls}\n`;
            
            if (stats.average_ring_time) {
                const avgRingSeconds = Math.round(stats.average_ring_time / 1000000000);
                message += `Average Ring Time: ${avgRingSeconds} seconds\n`;
            }
            
            if (stats.average_call_length) {
                const avgCallSeconds = Math.round(stats.average_call_length / 1000000000);
                message += `Average Call Length: ${avgCallSeconds} seconds\n`;
            }
            
            if (stats.busiest_member) {
                message += `Busiest Member: ${stats.busiest_member}\n`;
            }
            
            if (stats.last_call_time) {
                const lastCall = new Date(stats.last_call_time);
                message += `Last Call: ${lastCall.toLocaleString()}\n`;
            }
            
            alert(message);
        })
        .catch(error => {
            console.error('Error:', error);
            showAlert('Failed to load statistics', 'error');
        });
}

function removeMember(groupId, memberId) {
    confirmAction('Are you sure you want to remove this member from the hunt group?', () => {
        fetch(`/admin/huntgroups/${groupId}/members/${memberId}`, {
            method: 'DELETE'
        })
        .then(response => {
            if (response.ok) {
                showAlert('Member removed successfully', 'success');
                setTimeout(() => location.reload(), 1000);
            } else {
                showAlert('Failed to remove member', 'error');
            }
        })
        .catch(error => {
            console.error('Error:', error);
            showAlert('An error occurred while removing the member', 'error');
        });
    });
}

// Form validation
function validateForm(formId) {
    const form = document.getElementById(formId);
    if (!form) return true;
    
    const requiredFields = form.querySelectorAll('[required]');
    let isValid = true;
    
    requiredFields.forEach(field => {
        if (!field.value.trim()) {
            field.style.borderColor = '#dc3545';
            isValid = false;
        } else {
            field.style.borderColor = '#e9ecef';
        }
    });
    
    if (!isValid) {
        showAlert('Please fill in all required fields', 'error');
    }
    
    return isValid;
}

// Real-time validation
document.addEventListener('DOMContentLoaded', function() {
    // Add real-time validation to forms
    const forms = document.querySelectorAll('form');
    forms.forEach(form => {
        const requiredFields = form.querySelectorAll('[required]');
        requiredFields.forEach(field => {
            field.addEventListener('blur', function() {
                if (!this.value.trim()) {
                    this.style.borderColor = '#dc3545';
                } else {
                    this.style.borderColor = '#28a745';
                }
            });
            
            field.addEventListener('input', function() {
                if (this.value.trim()) {
                    this.style.borderColor = '#e9ecef';
                }
            });
        });
    });
    
    // Add loading states to buttons
    const buttons = document.querySelectorAll('button[type="submit"]');
    buttons.forEach(button => {
        button.addEventListener('click', function() {
            const form = this.closest('form');
            if (form && validateForm(form.id)) {
                this.textContent = 'Processing...';
                this.disabled = true;
            }
        });
    });
    
    // Auto-refresh statistics every 30 seconds on hunt group pages
    if (window.location.pathname.includes('/admin/huntgroups')) {
        setInterval(() => {
            const statsButtons = document.querySelectorAll('[onclick*="viewStatistics"]');
            if (statsButtons.length > 0) {
                // Silently refresh statistics in the background
                console.log('Auto-refreshing hunt group statistics...');
            }
        }, 30000);
    }
});

// Hunt Group specific functions
function showAddMemberForm() {
    const form = document.getElementById('add-member-form');
    if (form) {
        form.style.display = 'block';
        const extensionField = form.querySelector('#member_extension');
        if (extensionField) {
            extensionField.focus();
        }
    }
}

function hideAddMemberForm() {
    const form = document.getElementById('add-member-form');
    if (form) {
        form.style.display = 'none';
        form.reset();
    }
}

// Strategy description helper
function updateStrategyDescription() {
    const strategySelect = document.getElementById('strategy');
    const descriptionDiv = document.getElementById('strategy-description');
    
    if (!strategySelect || !descriptionDiv) return;
    
    const descriptions = {
        'simultaneous': 'All members will be called at the same time. The first to answer gets the call.',
        'sequential': 'Members will be called one by one in priority order until someone answers.',
        'round_robin': 'Members will be called in rotation, distributing calls evenly.',
        'longest_idle': 'The member who has been idle the longest will be called first.'
    };
    
    const selectedStrategy = strategySelect.value;
    descriptionDiv.textContent = descriptions[selectedStrategy] || '';
}

// Add event listener for strategy selection
document.addEventListener('DOMContentLoaded', function() {
    const strategySelect = document.getElementById('strategy');
    if (strategySelect) {
        strategySelect.addEventListener('change', updateStrategyDescription);
        updateStrategyDescription(); // Initial call
    }
});

// Keyboard shortcuts
document.addEventListener('keydown', function(e) {
    // Ctrl+N or Cmd+N for new item
    if ((e.ctrlKey || e.metaKey) && e.key === 'n') {
        e.preventDefault();
        const newButton = document.querySelector('a[href*="/new"]');
        if (newButton) {
            newButton.click();
        }
    }
    
    // Escape to cancel forms
    if (e.key === 'Escape') {
        const addMemberForm = document.getElementById('add-member-form');
        if (addMemberForm && addMemberForm.style.display !== 'none') {
            hideAddMemberForm();
        }
    }
});

// Export functions for global access
window.deleteUser = deleteUser;
window.deleteHuntGroup = deleteHuntGroup;
window.viewStatistics = viewStatistics;
window.removeMember = removeMember;
window.showAddMemberForm = showAddMemberForm;
window.hideAddMemberForm = hideAddMemberForm;