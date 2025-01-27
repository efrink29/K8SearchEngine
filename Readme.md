
# Distributed Web Crawler

This project is a distributed web crawler designed to crawl websites, track the frequency of words, and manage metadata of visited URLs. Built with **GoLang**, containerized using **Docker**, and deployed on **Google Cloud Kubernetes**, the system operates as a scalable and efficient solution for web crawling.

## Features

- **Distributed Crawling**: Multiple crawler containers work in parallel to scan web pages.
- **Word Frequency Analysis**: Tracks and indexes the frequency of words from the crawled pages.
- **URL Metadata Management**: Extracts and stores the title and description of each visited page.
- **Link Discovery**: Finds and manages links from crawled pages to enable recursive crawling.
- **Centralized Management**: A manager container handles communication with crawlers, manages the links database, and ensures efficient task distribution.

![Architecture Diagram](assets/WebCrawler1.png)

## Architecture Overview

1. **Manager Container**:
   - Receives crawl start requests via a specified port.
   - Manages the list of links to be crawled in the database.
   - Assigns URLs to crawler containers for processing.

2. **Crawler Containers**:
   - Retrieve assigned URLs from the manager.
   - Extract words and their frequencies from the webpage.
   - Add metadata (title and description) to the database.
   - Discover new links and send them back to the manager.

3. **Databases**:
   - **Metadata Database**: Stores the title and description of visited pages.
   - **Word Index Database**: Tracks the frequency of words across all crawled pages.
   - **Links Table**: Maintains a list of URLs to be crawled, ensuring no duplicates.

## Deployment

The system is deployed on **Google Cloud Kubernetes** for scalability and fault tolerance. Docker images for each component (manager and crawler) are pushed to Google Cloud Container Registry.

## Installation

### Prerequisites

1. [GoLang](https://golang.org/) installed on your local system.
2. [Docker](https://www.docker.com/) for containerization.
3. [kubectl](https://kubernetes.io/docs/tasks/tools/) for Kubernetes management.
4. Google Cloud account with Kubernetes Engine enabled.

### Steps

1. **Clone the Repository**:
   ```bash
   git clone <repository-url>
   cd <repository-folder>
   ```

2. **Build Docker Images**:
   ```bash
   docker build -t manager ./manager
   docker build -t crawler ./crawler
   ```

3. **Push Docker Images to Google Cloud Container Registry**:
   ```bash
   docker tag manager gcr.io/<your-project-id>/manager
   docker tag crawler gcr.io/<your-project-id>/crawler

   docker push gcr.io/<your-project-id>/manager
   docker push gcr.io/<your-project-id>/crawler
   ```

4. **Deploy to Kubernetes**:
   - Update `kubernetes/deployment.yaml` with your container image paths.
   - Apply the configurations:
     ```bash
     kubectl apply -f kubernetes/deployment.yaml
     ```

5. **Access the Manager**:
   - Expose the manager service using a LoadBalancer or port-forwarding:
     ```bash
     kubectl port-forward svc/manager 8080:8080
     ```
   - The manager can now receive requests at `http://localhost:8080`.

## Usage

1. **Start Crawling**:
   Send a POST request to the manager to start crawling from a specific URL:
   ```bash
   curl -X POST http://localhost:8080/start -d '{"url": "https://example.com"}'
   ```

2. **Monitor Progress**:
   Use Kubernetes logs to monitor the crawlers and manager:
   ```bash
   kubectl logs -l app=manager
   kubectl logs -l app=crawler
   ```

3. **Access Databases**:
   - The metadata, word index, and links tables are stored in a database accessible through the deployed service.

## Future Enhancements

- **Dynamic Scaling**: Implement autoscaling for crawler containers based on load.
- **Error Handling**: Improve handling of failed or unreachable URLs.
- **Dashboard**: Add a web-based dashboard to monitor crawl progress and visualize data.

