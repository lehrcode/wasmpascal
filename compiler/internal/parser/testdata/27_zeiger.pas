program Zeiger;
type
  T = Integer;
  P = ^T;
  PP = ^P;
  PStr = ^string;
  PRec = ^TRec;
  TRec = record
    Wert: T;
    Next: PRec
  end;
var
  a: T;
  p1: P;
  pp1: PP;
  ps: PStr;
  r: PRec;
  Roh: Pointer;
begin
  a := 5;
  p1 := @a;
  pp1 := @p1;
  pp1^^ := 7;
  WriteLn(a, ' ', p1^);
  New(ps);
  ps^ := 'Zeiger auf String';
  WriteLn(ps^, ' ', Length(ps^));
  Dispose(ps);
  New(r);
  r^.Next := nil;
  r^.Wert := pp1^^ + 1;
  Roh := r;
  if PRec(Roh)^.Next = nil then WriteLn(PRec(Roh)^.Wert);
  GetMem(Roh, 100);
  FreeMem(Roh, 100);
  p1 := Ptr(0, Ofs(a));
  WriteLn(Seg(a), Addr(a) = @a)
end.
