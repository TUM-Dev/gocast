import {Injectable} from '@angular/core';
import {HttpClient} from "@angular/common/http";
import WatchDto from "../dto/watch.dto";
import {Observable} from "rxjs";
import {HTTPClientService} from "../common/httpclient.service";

@Injectable({
  providedIn: 'root'
})
export class WatchService {
  private readonly endpoint = '/api/watch/';

  constructor(private http: HTTPClientService) {

  }

  getWatchPage(streamId: string | null): Observable<WatchDto> {
    return this.http.get<WatchDto>(this.endpoint + streamId);
  }
}
