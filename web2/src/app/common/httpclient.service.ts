import {Injectable} from '@angular/core';
import {HttpClient} from '@angular/common/http';
import {Observable} from "rxjs";

@Injectable({
  providedIn: 'root'
})

export class HTTPClientService {

  constructor(private httpClient: HttpClient) {
  }

  getCredentials(): string | undefined {
    const storedCred = localStorage.getItem('token');
    if (storedCred) {
      return storedCred;
    }
    return undefined;
  }

  get<T>(endpoint: string): Observable<T> {
    let headers = {}
    if (this.getCredentials()) {
      headers = {
        'Authorization': this.getCredentials()
      }
    }
    return this.httpClient.get<T>(endpoint, {
      headers: {
        'Authorization': 'Bearer ' + this.getCredentials()
      }
    });
  }

  post<T>(endpoint: string, body: any): Observable<T> {
    return this.httpClient.post<T>(endpoint, body, {
      headers: {
        'Authorization': 'Bearer ' + this.getCredentials()
      }
    });
  }
}
