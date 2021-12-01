import {Injectable} from '@angular/core';
import {HttpClient} from "@angular/common/http";
import WatchDto from "../dto/watch.dto";
import {Observable} from "rxjs";

@Injectable({
  providedIn: 'root'
})
export class WatchService {
  private readonly baseUrl = 'http://localhost:8081/api/watch/';

  constructor(private http: HttpClient) {
  }

  getWatchPage(streamId: string | null): Observable<WatchDto> {
    return this.http.get<WatchDto>(this.baseUrl + streamId);
  }
}
