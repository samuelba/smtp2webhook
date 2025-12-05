#!/bin/bash
# Helper script to test SMTP Webhook Forwarder with Docker Compose

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

# Check if docker compose is available
if ! docker compose version &> /dev/null; then
    print_error "docker compose is not available. Please install Docker with Compose V2."
    exit 1
fi

# Function to check if services are running
check_services() {
    print_info "Checking service status..."
    docker compose ps
}

# Function to send test email
send_test_email() {
    local to=$1
    local from=${2:-"sender@test.com"}
    local subject=${3:-"Test Email"}
    local body=${4:-"This is a test email from the SMTP Webhook Forwarder test script."}
    
    print_info "Sending test email to: $to"
    
    if command -v swaks &> /dev/null; then
        swaks --to "$to" \
              --from "$from" \
              --server localhost:2525 \
              --body "$body" \
              --header "Subject: $subject" \
              --suppress-data
    else
        print_warning "swaks is not installed. Using telnet method..."
        print_info "You can install swaks with: apt-get install swaks (Debian/Ubuntu) or brew install swaks (macOS)"
        
        # Alternative using nc (netcat)
        if command -v nc &> /dev/null; then
            (
                echo "HELO test.com"
                sleep 0.5
                echo "MAIL FROM:<$from>"
                sleep 0.5
                echo "RCPT TO:<$to>"
                sleep 0.5
                echo "DATA"
                sleep 0.5
                echo "Subject: $subject"
                echo "From: $from"
                echo "To: $to"
                echo ""
                echo "$body"
                echo "."
                sleep 0.5
                echo "QUIT"
            ) | nc localhost 2525
        else
            print_error "Neither swaks nor nc is installed. Cannot send test email."
            return 1
        fi
    fi
}

# Function to view logs
view_logs() {
    local service=${1:-"smtp-forwarder"}
    print_info "Viewing logs for $service (Ctrl+C to exit)..."
    docker compose logs -f "$service"
}

# Main menu
show_menu() {
    echo ""
    echo "=================================="
    echo "SMTP Webhook Forwarder Test Menu"
    echo "=================================="
    echo "1. Start services"
    echo "2. Stop services"
    echo "3. Check service status"
    echo "4. Send test email (exact match: test@example.com)"
    echo "5. Send test email (wildcard match: anything@example.com)"
    echo "6. Send test email (multiple matches: support@example.com)"
    echo "7. Send test email (default webhook: unknown@other.com)"
    echo "8. View SMTP forwarder logs"
    echo "9. View webhook receiver logs"
    echo "10. View all logs"
    echo "11. Generate TLS certificates"
    echo "12. Restart services"
    echo "0. Exit"
    echo "=================================="
}

# Process menu choice
process_choice() {
    local choice=$1
    
    case $choice in
        1)
            print_info "Starting services..."
            docker compose up -d
            sleep 3
            check_services
            ;;
        2)
            print_info "Stopping services..."
            docker compose down
            ;;
        3)
            check_services
            ;;
        4)
            send_test_email "test@example.com"
            ;;
        5)
            send_test_email "anything@example.com"
            ;;
        6)
            send_test_email "support@example.com"
            ;;
        7)
            send_test_email "unknown@other.com"
            ;;
        8)
            view_logs "smtp-forwarder"
            ;;
        9)
            view_logs "webhook-receiver"
            ;;
        10)
            print_info "Viewing all logs (Ctrl+C to exit)..."
            docker compose logs -f
            ;;
        11)
            print_info "Generating TLS certificates..."
            mkdir -p certs
            openssl req -x509 -newkey rsa:4096 \
                -keyout certs/server.key \
                -out certs/server.crt \
                -days 365 -nodes \
                -subj "/CN=smtp-forwarder"
            print_info "Certificates generated in ./certs/"
            print_warning "Remember to update docker-compose.yml to use config.tls.example.json"
            ;;
        12)
            print_info "Restarting services..."
            docker compose restart
            sleep 3
            check_services
            ;;
        0)
            print_info "Exiting..."
            exit 0
            ;;
        *)
            print_error "Invalid choice. Please try again."
            ;;
    esac
}

# Main loop
main() {
    if [ $# -eq 0 ]; then
        # Interactive mode
        while true; do
            show_menu
            read -p "Enter your choice: " choice
            process_choice "$choice"
            echo ""
            read -p "Press Enter to continue..."
        done
    else
        # Command line mode
        process_choice "$1"
    fi
}

main "$@"
